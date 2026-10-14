package dfir

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/aprksy/knitknot/extensions"
	"github.com/aprksy/knitknot/pkg/ports/storage"
	"github.com/aprksy/knitknot/pkg/ports/types"
)

// EvidenceStats counts what ImportEvidence did: Added nodes created,
// Merged observations folded into an existing node (history appended,
// never duplicated).
type EvidenceStats struct {
	Added  int `json:"added"`
	Merged int `json:"merged"`
}

// ImportEvidence persists case entities as observable nodes — the same
// layer the workspace produces (props type/value/name/order/source_feed)
// — so evidence accumulates in the graph across SIEM windows instead of
// living in a one-shot CSV.
//
// Identity is kind + canonical value: observables use Observable.Key()
// semantics, techniques/malware/tool a trimmed/lowercased name key
// (coverage matches names case-insensitively). The stored value keeps the
// display form (canonical for observables, original case otherwise); name
// carries the canonical key.
//
// Merge: an existing identity keeps its display props and source_feed
// (first-seen wins) while order drops to the minimum (first-seen) and a
// snapshot appends — so re-importing a window adds no nodes and "seen in
// W1 and W3" stays in the node's history. Source is recorded per
// observation in the snapshot where the backend supports it.
func ImportEvidence(s storage.StorageEngine, entities []CaseEntity, source string) (EvidenceStats, error) {
	var stats EvidenceStats
	byIdentity := make(map[string]*types.Node)
	for _, n := range s.GetNodesByLabel(LabelObservable) {
		if id, ok := evidenceIdentityOf(n); ok {
			if _, dup := byIdentity[id]; !dup {
				byIdentity[id] = n
			}
		}
	}

	for _, e := range entities {
		kind, key, display := evidenceKeys(e)
		id := kind + "\x00" + key
		if n, found := byIdentity[id]; found {
			props := cloneProps(n.Props)
			if cur, ok := asRank(props["order"]); !ok || e.Order < cur {
				props["order"] = e.Order
			}
			if err := updateEvidence(s, n.ID, props, source); err != nil {
				return stats, fmt.Errorf("dfir: evidence: merge %q: %w", e.Value, err)
			}
			stats.Merged++
			continue
		}
		nid, err := addEvidence(s, map[string]any{
			"type":         kind,
			"value":        display,
			"name":         key,
			"order":        e.Order,
			PropSourceFeed: source,
		}, source)
		if err != nil {
			return stats, fmt.Errorf("dfir: evidence: add %q: %w", e.Value, err)
		}
		if fresh, ok := s.GetNode(nid); ok {
			byIdentity[id] = fresh
		}
		stats.Added++
	}
	return stats, nil
}

// EvidenceFromGraph reads accumulated evidence back out of the graph's
// observable layer as case entities (type→kind, value, order). With
// source != "" only nodes first seen in that window (source_feed) match;
// "" reads all windows. Nodes missing type or value are skipped.
// Sorted by (order, kind, value) so ordered coverage is deterministic.
// Context is not persisted on evidence nodes (v1), so it reads back empty.
func EvidenceFromGraph(s storage.StorageEngine, source string) []CaseEntity {
	var out []CaseEntity
	for _, n := range s.GetNodesByLabel(LabelObservable) {
		kind, _ := n.Props["type"].(string)
		val, _ := n.Props["value"].(string)
		if kind == "" || strings.TrimSpace(val) == "" {
			continue
		}
		if source != "" {
			if sf, _ := n.Props[PropSourceFeed].(string); sf != source {
				continue
			}
		}
		order := 0
		if o, ok := asRank(n.Props["order"]); ok {
			order = o
		}
		out = append(out, CaseEntity{Kind: kind, Value: strings.TrimSpace(val), Order: order})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Order != out[j].Order {
			return out[i].Order < out[j].Order
		}
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Value < out[j].Value
	})
	return out
}

// evidenceKeys splits an entity into its node kind, canonical identity
// key, and display value. Techniques, malware, and tool names match
// case-insensitively, so their keys are lowercased while display keeps the
// original form.
func evidenceKeys(e CaseEntity) (kind, key, display string) {
	switch e.Kind {
	case EntityTechnique, EntityMalware, EntityTool:
		return e.Kind, strings.ToLower(strings.TrimSpace(e.Value)), strings.TrimSpace(e.Value)
	}
	o := Observable{Type: e.Kind, Value: e.Value}
	return e.Kind, o.Canonical(), o.Canonical()
}

// evidenceIdentityOf returns the identity key of an observable node
// (type + name, the canonical key), or false when the node is not shaped
// like evidence.
func evidenceIdentityOf(n *types.Node) (string, bool) {
	kind, _ := n.Props["type"].(string)
	key, _ := n.Props["name"].(string)
	if kind == "" || key == "" {
		return "", false
	}
	return kind + "\x00" + key, true
}

// evidenceMetaStore is the optional versioning write path carrying
// source/transaction. Backends without it fall back to plain writes.
type evidenceMetaStore interface {
	AddNodeWithMeta(label string, props map[string]any, source, transaction string) (string, error)
	UpdateNodeWithMeta(id string, props map[string]any, source, transaction string) error
}

func addEvidence(s storage.StorageEngine, props map[string]any, source string) (string, error) {
	if ms, ok := s.(evidenceMetaStore); ok {
		return ms.AddNodeWithMeta(LabelObservable, props, source, "")
	}
	return s.AddNode(LabelObservable, props)
}

func updateEvidence(s storage.StorageEngine, id string, props map[string]any, source string) error {
	if ms, ok := s.(evidenceMetaStore); ok {
		return ms.UpdateNodeWithMeta(id, props, source, "")
	}
	return s.UpdateNode(id, props)
}

// EvidenceImporter is an extension.Importer that reads a case-entity CSV
// and accumulates it into the graph via ImportEvidence, so
// `import --format evidence --source <window>` works.
type EvidenceImporter struct{}

// NewEvidenceImporter returns the evidence importer.
func NewEvidenceImporter() *EvidenceImporter { return &EvidenceImporter{} }

var _ extension.Importer = (*EvidenceImporter)(nil)

// Import parses the case-entity CSV and merges it into storage under
// ic.Source, reporting added vs merged nodes.
func (im *EvidenceImporter) Import(ctx context.Context, ic extension.ImportContext, s storage.StorageEngine, verbs *types.VerbRegistry, r io.Reader) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	_ = verbs
	entities, err := ParseCaseEntities(r)
	if err != nil {
		return err
	}
	stats, err := ImportEvidence(s, entities, ic.Source)
	if err != nil {
		return err
	}
	fmt.Printf("-- Evidence: %d added, %d merged (source %s)\n", stats.Added, stats.Merged, ic.Source)
	return nil
}

// Extension wires the DFIR evidence importer into the registry.
type Extension struct{}

// New returns the DFIR extension.
func New() *Extension { return &Extension{} }

var _ extension.Extension = (*Extension)(nil)

// Name identifies the extension.
func (e *Extension) Name() string { return "dfir" }

// Register adds the evidence importer.
func (e *Extension) Register(r extension.Registry) error {
	return r.RegisterImporter("evidence", NewEvidenceImporter())
}
