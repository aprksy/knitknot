// ext: cti-import — STIX 2.1 bundle importer (two-pass: stage, then upsert).
package cti

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/TcM1911/stix2"
	"github.com/aprksy/knitknot/extensions"
	"github.com/aprksy/knitknot/pkg/ports/storage"
	"github.com/aprksy/knitknot/pkg/ports/types"
)

// STIXImporter imports STIX 2.1 bundles into the graph. // ext: cti-import
type STIXImporter struct{} // ext: cti-import

// NewSTIXImporter returns a STIX 2.1 bundle importer. // ext: cti-import
func NewSTIXImporter() *STIXImporter { return &STIXImporter{} } // ext: cti-import

var _ extension.Importer = (*STIXImporter)(nil) // ext: cti-import

type stagedNode struct { // ext: cti-import
	stixID string         // ext: cti-import
	label  string         // ext: cti-import
	props  map[string]any // ext: cti-import
} // ext: cti-import

type stagedRel struct { // ext: cti-import
	stixID  string         // ext: cti-import
	kind    string         // ext: cti-import
	fromRef string         // ext: cti-import
	toRef   string         // ext: cti-import
	props   map[string]any // ext: cti-import
} // ext: cti-import

// Import reads a STIX 2.1 bundle and upserts its objects as nodes and edges. // ext: cti-import
// Pass 1 stages and validates everything without touching storage; pass 2 // ext: cti-import
// upserts nodes first, then resolves and upserts relationships. // ext: cti-import
func (im *STIXImporter) Import(ctx context.Context, ic extension.ImportContext, s storage.StorageEngine, verbs *types.VerbRegistry, r io.Reader) error { // ext: cti-import
	_ = verbs                  // ext: cti-import
	data, err := io.ReadAll(r) // ext: cti-import
	if err != nil {            // ext: cti-import
		return fmt.Errorf("cti: read bundle: %w", err) // ext: cti-import
	} // ext: cti-import
	collection, err := stix2.FromJSON(data) // ext: cti-import
	if err != nil {                         // ext: cti-import
		return fmt.Errorf("cti: parse STIX bundle: %w", err) // ext: cti-import
	} // ext: cti-import

	// Pass 1: stage and validate, no storage mutation. // ext: cti-import
	var nodes []stagedNode                        // ext: cti-import
	var rels []stagedRel                          // ext: cti-import
	staged := make(map[string]struct{})           // ext: cti-import
	for i, obj := range collection.AllObjects() { // ext: cti-import
		if i%64 == 0 { // ext: cti-import
			if err := ctx.Err(); err != nil { // ext: cti-import
				return err // ext: cti-import
			} // ext: cti-import
		} // ext: cti-import
		stixID := string(obj.GetID()) // ext: cti-import
		if stixID == "" {             // ext: cti-import
			return fmt.Errorf("cti: object missing STIX id") // ext: cti-import
		} // ext: cti-import
		if _, _, err := ParseSTIXID(stixID); err != nil { // ext: cti-import
			return fmt.Errorf("cti: invalid STIX id %q: %w", stixID, err) // ext: cti-import
		} // ext: cti-import
		if rel, ok := obj.(*stix2.Relationship); ok { // ext: cti-import
			if rel.Source == "" || rel.Target == "" { // ext: cti-import
				return fmt.Errorf("cti: relationship %q missing source_ref or target_ref", stixID) // ext: cti-import
			} // ext: cti-import
			if rel.RelationshipType == "" { // ext: cti-import
				return fmt.Errorf("cti: relationship %q missing relationship_type", stixID) // ext: cti-import
			} // ext: cti-import
			props, err := graphProps(obj, ic.Source) // ext: cti-import // ext: source-feed
			if err != nil {                          // ext: cti-import
				return fmt.Errorf("cti: relationship %q: %w", stixID, err) // ext: cti-import
			} // ext: cti-import
			rels = append(rels, stagedRel{stixID: stixID, kind: string(rel.RelationshipType), fromRef: string(rel.Source), toRef: string(rel.Target), props: props}) // ext: cti-import
			continue                                                                                                                                                 // ext: cti-import
		} // ext: cti-import
		if ind, ok := obj.(*stix2.Indicator); ok { // ext: cti-import
			if ind.Pattern == "" || ind.PatternType == "" || ind.ValidFrom == nil { // ext: cti-import
				return fmt.Errorf("cti: indicator %q missing required pattern, pattern_type, or valid_from", stixID) // ext: cti-import
			} // ext: cti-import
		} // ext: cti-import
		props, err := graphProps(obj, ic.Source) // ext: cti-import // ext: source-feed
		if err != nil {                          // ext: cti-import
			return fmt.Errorf("cti: object %q: %w", stixID, err) // ext: cti-import
		} // ext: cti-import
		// ext: cti-import — label is the STIX type verbatim; supported SDOs match vocabulary consts, anything else (SCOs, x-custom) is preserved as-is.
		nodes = append(nodes, stagedNode{stixID: stixID, label: string(obj.GetType()), props: props}) // ext: cti-import
		staged[stixID] = struct{}{}                                                                   // ext: cti-import
	} // ext: cti-import

	// Still pass 1: every relationship endpoint must resolve against the staged // ext: cti-import
	// set or pre-existing storage before any mutation happens. // ext: cti-import
	for _, rel := range rels { // ext: cti-import
		for _, ref := range []string{rel.fromRef, rel.toRef} { // ext: cti-import
			if _, ok := staged[ref]; ok { // ext: cti-import
				continue // ext: cti-import
			} // ext: cti-import
			if findGraphID(s, ref) == "" { // ext: cti-import
				return fmt.Errorf("cti: relationship %q references unknown object %q", rel.stixID, ref) // ext: cti-import
			} // ext: cti-import
		} // ext: cti-import
	} // ext: cti-import

	// Pass 2: upsert nodes, then resolve and upsert relationships. // ext: cti-import
	// ponytail: single-writer lookup-before-add; needs a unique index for concurrent/large feeds.
	// perf: preload stixID->node once per involved label; the old per-node
	// label-bucket scan was O(nodes*bucket) and never hit on fresh import.
	labels := make(map[string]struct{}, 8) // ext: cti-import
	for _, n := range nodes {              // ext: cti-import
		labels[n.label] = struct{}{} // ext: cti-import
	} // ext: cti-import
	known := make(map[string]*types.Node, len(nodes)) // ext: cti-import
	for label := range labels {                       // ext: cti-import
		for _, n := range s.GetNodesByLabel(label) { // ext: cti-import
			if sid, _ := n.Props[StixID].(string); sid != "" { // ext: cti-import
				known[sid] = n // ext: cti-import
			} // ext: cti-import
		} // ext: cti-import
	} // ext: cti-import
	ids := make(map[string]string, len(nodes)) // ext: cti-import
	skipped := 0                               // ext: cti-import — stale re-imports skipped below
	pass2 := func() error {                    // ext: cti-import
		for i, n := range nodes { // ext: cti-import
			if i%64 == 0 { // ext: cti-import
				if err := ctx.Err(); err != nil { // ext: cti-import
					return err // ext: cti-import
				} // ext: cti-import
			} // ext: cti-import
			if existing, ok := known[n.stixID]; ok { // ext: cti-import
				if isStale(existing.Props, n.props) { // ext: cti-import — older re-import: keep live props, no new snapshot
					skipped++                   // ext: cti-import
					ids[n.stixID] = existing.ID // ext: cti-import
					continue                    // ext: cti-import
				} // ext: cti-import
				if err := updateNodeSrc(s, existing.ID, mergeProps(existing.Props, n.props), ic.Source, ic.Transaction); err != nil { // ext: cti-import // ext: source-feed
					return fmt.Errorf("cti: update node %q: %w", n.stixID, err) // ext: cti-import
				} // ext: cti-import
				ids[n.stixID] = existing.ID // ext: cti-import
				continue                    // ext: cti-import
			} // ext: cti-import
			gid, err := addNodeSrc(s, n.label, n.props, ic.Source, ic.Transaction) // ext: cti-import // ext: source-feed
			if err != nil {                                                        // ext: cti-import
				return fmt.Errorf("cti: add node %q: %w", n.stixID, err) // ext: cti-import
			} // ext: cti-import
			ids[n.stixID] = gid                  // ext: cti-import
			if fresh, ok := s.GetNode(gid); ok { // ext: cti-import
				known[n.stixID] = fresh // ext: cti-import — dup stixIDs in one bundle merge
			} // ext: cti-import
		} // ext: cti-import
		// perf: seen-set for edges added this import + one GetEdgesFrom per
		// from-node; the old per-relationship scan was quadratic on hub nodes.
		seen := make(map[string]string, len(rels))           // ext: cti-import — from|to|kind -> edge ID
		fromCache := make(map[string]map[string]*types.Edge) // ext: cti-import — from -> to|kind -> edge
		for i, rel := range rels {                           // ext: cti-import
			if i%64 == 0 { // ext: cti-import
				if err := ctx.Err(); err != nil { // ext: cti-import
					return err // ext: cti-import
				} // ext: cti-import
			} // ext: cti-import
			from := ids[rel.fromRef] // ext: cti-import
			if from == "" {          // ext: cti-import
				from = resolveGraphID(s, known, rel.fromRef) // ext: cti-import
			} // ext: cti-import
			to := ids[rel.toRef] // ext: cti-import
			if to == "" {        // ext: cti-import
				to = resolveGraphID(s, known, rel.toRef) // ext: cti-import
			} // ext: cti-import
			if from == "" || to == "" { // ext: cti-import
				return fmt.Errorf("cti: relationship %q has unresolvable endpoint", rel.stixID) // ext: cti-import
			} // ext: cti-import
			key := from + "\x00" + to + "\x00" + rel.kind // ext: cti-import
			if eid, ok := seen[key]; ok {                 // ext: cti-import — dup in this import
				if e, ok := s.GetEdge(eid); ok { // ext: cti-import
					if isStale(e.Props, rel.props) { // ext: cti-import
						skipped++ // ext: cti-import
						continue  // ext: cti-import
					} // ext: cti-import
					if err := updateEdgeSrc(s, e.ID, mergeProps(e.Props, rel.props), ic.Source, ic.Transaction); err != nil { // ext: cti-import // ext: source-feed
						return fmt.Errorf("cti: update edge %q: %w", rel.stixID, err) // ext: cti-import
					} // ext: cti-import
					continue // ext: cti-import
				} // ext: cti-import
			} // ext: cti-import
			bucket, ok := fromCache[from] // ext: cti-import
			if !ok {                      // ext: cti-import
				bucket = make(map[string]*types.Edge)    // ext: cti-import
				for _, e := range s.GetEdgesFrom(from) { // ext: cti-import
					bucket[e.To+"\x00"+e.Kind] = e // ext: cti-import
				} // ext: cti-import
				fromCache[from] = bucket // ext: cti-import
			} // ext: cti-import
			if found, ok := bucket[to+"\x00"+rel.kind]; ok { // ext: cti-import
				if isStale(found.Props, rel.props) { // ext: cti-import
					seen[key] = found.ID // ext: cti-import
					skipped++            // ext: cti-import
					continue             // ext: cti-import
				} // ext: cti-import
				if err := updateEdgeSrc(s, found.ID, mergeProps(found.Props, rel.props), ic.Source, ic.Transaction); err != nil { // ext: cti-import // ext: source-feed
					return fmt.Errorf("cti: update edge %q: %w", rel.stixID, err) // ext: cti-import
				} // ext: cti-import
				seen[key] = found.ID // ext: cti-import
				continue             // ext: cti-import
			} // ext: cti-import
			if err := addEdgeSrc(s, from, to, rel.kind, rel.props, ic.Source, ic.Transaction); err != nil { // ext: cti-import // ext: source-feed
				return fmt.Errorf("cti: add edge %q: %w", rel.stixID, err) // ext: cti-import
			} // ext: cti-import
			// perf: resolve the real edge (O(1) on deterministic IDs, one scan
			// fallback otherwise) so later dups hit seen->GetEdge, not a re-add.
			if e, ok := s.GetEdge(from + "->" + to + "@" + rel.kind); ok { // ext: cti-import
				seen[key] = e.ID               // ext: cti-import
				bucket[to+"\x00"+rel.kind] = e // ext: cti-import
				continue                       // ext: cti-import
			} // ext: cti-import
			for _, e := range s.GetEdgesFrom(from) { // ext: cti-import — non-deterministic backend fallback
				if e.To == to && e.Kind == rel.kind { // ext: cti-import
					seen[key] = e.ID               // ext: cti-import
					bucket[to+"\x00"+rel.kind] = e // ext: cti-import
					break                          // ext: cti-import
				} // ext: cti-import
			} // ext: cti-import
		} // ext: cti-import
		return nil // ext: cti-import
	} // ext: cti-import
	if bs, ok := s.(batchStore); ok { // ext: cti-import
		if err := bs.Batch(pass2); err != nil { // ext: cti-import
			return err // ext: cti-import
		} // ext: cti-import
	} else if err := pass2(); err != nil { // ext: cti-import
		return err // ext: cti-import
	} // ext: cti-import
	if skipped > 0 { // ext: cti-import
		fmt.Fprintf(os.Stderr, "cti: skipped %d stale object(s) older than stored snapshots\n", skipped) // ext: cti-import
	} // ext: cti-import
	return nil // ext: cti-import
} // ext: cti-import

// graphProps flattens a STIX object to generic node/edge props via its JSON // ext: cti-import
// form, so known fields and raw custom properties survive without a // ext: cti-import
// per-type field list. // ext: cti-import
func graphProps(obj stix2.STIXObject, source string) (map[string]any, error) { // ext: cti-import // ext: source-feed
	raw, err := json.Marshal(obj) // ext: cti-import
	if err != nil {               // ext: cti-import
		return nil, err // ext: cti-import
	} // ext: cti-import
	props := make(map[string]any)                       // ext: cti-import
	if err := json.Unmarshal(raw, &props); err != nil { // ext: cti-import
		return nil, err // ext: cti-import
	} // ext: cti-import
	props[StixID] = string(obj.GetID())                       // ext: cti-import
	delete(props, "id")                                       // ext: cti-import
	props[SourceFeed] = source                                // ext: cti-import // ext: source-feed
	props[ImportedAt] = time.Now().UTC().Format(time.RFC3339) // ext: cti-import
	return props, nil                                         // ext: cti-import
} // ext: cti-import

type batchStore interface { // ext: cti-import
	Batch(fn func() error) error // ext: cti-import
} // ext: cti-import

// metaStore is the optional versioning write path carrying source/transaction. // ext: source-feed
// Backends without it fall back to plain StorageEngine writes (Source lost). // ext: source-feed
type metaStore interface { // ext: source-feed
	AddNodeWithMeta(label string, props map[string]any, source, transaction string) (string, error) // ext: source-feed
	AddEdgeWithMeta(from, to, kind string, props map[string]any, source, transaction string) error  // ext: source-feed
	UpdateNodeWithMeta(id string, props map[string]any, source, transaction string) error           // ext: source-feed
	UpdateEdgeWithMeta(id string, props map[string]any, source, transaction string) error           // ext: source-feed
} // ext: source-feed

func addNodeSrc(s storage.StorageEngine, label string, props map[string]any, source, tx string) (string, error) { // ext: source-feed
	if ms, ok := s.(metaStore); ok { // ext: source-feed
		return ms.AddNodeWithMeta(label, props, source, tx) // ext: source-feed
	} // ext: source-feed
	return s.AddNode(label, props) // ext: source-feed
} // ext: source-feed

func addEdgeSrc(s storage.StorageEngine, from, to, kind string, props map[string]any, source, tx string) error { // ext: source-feed
	if ms, ok := s.(metaStore); ok { // ext: source-feed
		return ms.AddEdgeWithMeta(from, to, kind, props, source, tx) // ext: source-feed
	} // ext: source-feed
	return s.AddEdge(from, to, kind, props) // ext: source-feed
} // ext: source-feed

func updateNodeSrc(s storage.StorageEngine, id string, props map[string]any, source, tx string) error { // ext: source-feed
	if ms, ok := s.(metaStore); ok { // ext: source-feed
		return ms.UpdateNodeWithMeta(id, props, source, tx) // ext: source-feed
	} // ext: source-feed
	return s.UpdateNode(id, props) // ext: source-feed
} // ext: source-feed

func updateEdgeSrc(s storage.StorageEngine, id string, props map[string]any, source, tx string) error { // ext: source-feed
	if ms, ok := s.(metaStore); ok { // ext: source-feed
		return ms.UpdateEdgeWithMeta(id, props, source, tx) // ext: source-feed
	} // ext: source-feed
	return s.UpdateEdge(id, props) // ext: source-feed
} // ext: source-feed

// resolveGraphID maps a STIX ID to its graph ID via the Pass 2 preload,
// falling back to a full scan for pre-existing nodes whose label was not
// part of this import. // ext: cti-import
func resolveGraphID(s storage.StorageEngine, known map[string]*types.Node, stixID string) string { // ext: cti-import
	if n, ok := known[stixID]; ok { // ext: cti-import
		return n.ID // ext: cti-import
	} // ext: cti-import
	return findGraphID(s, stixID) // ext: cti-import
} // ext: cti-import

// findGraphID scans all nodes for stixID and returns its graph ID, or "". // ext: cti-import
func findGraphID(s storage.StorageEngine, stixID string) string { // ext: cti-import
	for _, n := range s.GetAllNodes() { // ext: cti-import
		if v, _ := n.Props[StixID].(string); v == stixID { // ext: cti-import
			return n.ID // ext: cti-import
		} // ext: cti-import
	} // ext: cti-import
	return "" // ext: cti-import
} // ext: cti-import

// isStale reports whether newProps predates oldProps by STIX `modified`. // ext: cti-import
// Missing or unparseable timestamps on either side mean "can't tell" — // ext: cti-import
// not stale, so the update proceeds as before. Equal timestamps proceed. // ext: cti-import
func isStale(oldProps, newProps map[string]any) bool { // ext: cti-import
	oldT, ok := stixTime(oldProps["modified"]) // ext: cti-import
	if !ok {                                   // ext: cti-import
		return false // ext: cti-import
	} // ext: cti-import
	newT, ok := stixTime(newProps["modified"]) // ext: cti-import
	if !ok {                                   // ext: cti-import
		return false // ext: cti-import
	} // ext: cti-import
	return newT.Before(oldT) // ext: cti-import
} // ext: cti-import

func stixTime(v any) (time.Time, bool) { // ext: cti-import
	s, ok := v.(string) // ext: cti-import
	if !ok || s == "" { // ext: cti-import
		return time.Time{}, false // ext: cti-import
	} // ext: cti-import
	t, err := time.Parse(time.RFC3339Nano, s) // ext: cti-import
	if err != nil {                           // ext: cti-import
		return time.Time{}, false // ext: cti-import
	} // ext: cti-import
	return t, true // ext: cti-import
} // ext: cti-import

// mergeProps overlays new on old without mutating either input. // ext: cti-import
func mergeProps(old, new map[string]any) map[string]any { // ext: cti-import
	out := make(map[string]any, len(old)+len(new)) // ext: cti-import
	for k, v := range old {                        // ext: cti-import
		out[k] = v // ext: cti-import
	} // ext: cti-import
	for k, v := range new { // ext: cti-import
		out[k] = v // ext: cti-import
	} // ext: cti-import
	return out // ext: cti-import
} // ext: cti-import
