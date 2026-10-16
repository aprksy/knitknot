package memory

import (
	"context"
	"fmt"

	"github.com/aprksy/knitknot/extensions"
	"github.com/aprksy/knitknot/pkg/ports/storage"
	"github.com/aprksy/knitknot/pkg/ports/types"
	"io"
)

// MemoryImporter imports memory-bundle v1 documents into the graph.
type MemoryImporter struct{}

// NewMemoryImporter returns a memory-bundle importer.
func NewMemoryImporter() *MemoryImporter { return &MemoryImporter{} }

var _ extension.Importer = (*MemoryImporter)(nil)

// Import upserts bundle nodes by memory_id (merge props, append a snapshot)
// then resolves edges via memory_id. Validation and dangling-ref checks run
// before any mutation, so a bad bundle aborts with zero partial writes.
func (im *MemoryImporter) Import(ctx context.Context, ic extension.ImportContext, s storage.StorageEngine, verbs *types.VerbRegistry, r io.Reader) error {
	_ = verbs
	b, err := ParseBundle(r)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	// Still pass 1: every edge endpoint must resolve against the staged set
	// or pre-existing storage before any mutation happens.
	staged := make(map[string]struct{}, len(b.Nodes))
	for _, n := range b.Nodes {
		staged[n.MemoryID] = struct{}{}
	}
	for _, e := range b.Edges {
		for _, ref := range []string{e.From, e.To} {
			if _, ok := staged[ref]; ok {
				continue
			}
			if findGraphID(s, ref) == "" {
				return fmt.Errorf("memory: edge (%s->%s) references unknown memory_id %q", e.From, e.To, ref)
			}
		}
	}

	// Pass 2: upsert nodes, then resolve and upsert edges.
	// ponytail: single-writer lookup-before-add with a per-label preload; needs a unique index for concurrent/large feeds.
	labels := make(map[string]struct{}, 8)
	for _, n := range b.Nodes {
		labels[n.Label] = struct{}{}
	}
	known := make(map[string]*types.Node, len(b.Nodes))
	for label := range labels {
		for _, n := range s.GetNodesByLabel(label) {
			if sid, _ := n.Props[PropMemoryID].(string); sid != "" {
				known[sid] = n
			}
		}
	}
	pass2 := func() error {
		ids := make(map[string]string, len(b.Nodes))
		for i, n := range b.Nodes {
			if i%64 == 0 {
				if err := ctx.Err(); err != nil {
					return err
				}
			}
			props := mergeProps(nil, n.Props)
			props[PropMemoryID] = n.MemoryID
			if existing, ok := known[n.MemoryID]; ok {
				if err := updateNodeSrc(s, existing.ID, mergeProps(existing.Props, props), ic.Source, ic.Transaction); err != nil {
					return fmt.Errorf("memory: update node %q: %w", n.MemoryID, err)
				}
				ids[n.MemoryID] = existing.ID
				continue
			}
			gid, err := addNodeSrc(s, n.Label, props, ic.Source, ic.Transaction)
			if err != nil {
				return fmt.Errorf("memory: add node %q: %w", n.MemoryID, err)
			}
			ids[n.MemoryID] = gid
			if fresh, ok := s.GetNode(gid); ok {
				known[n.MemoryID] = fresh // dup memory_ids in one bundle merge
			}
		}
		// ponytail: O(edges*out-degree) edge scan per import; cache GetEdgesFrom per from-node if hub nodes get slow.
		for i, e := range b.Edges {
			if i%64 == 0 {
				if err := ctx.Err(); err != nil {
					return err
				}
			}
			from := ids[e.From]
			if from == "" {
				from = resolveGraphID(s, known, e.From)
			}
			to := ids[e.To]
			if to == "" {
				to = resolveGraphID(s, known, e.To)
			}
			if from == "" || to == "" {
				return fmt.Errorf("memory: edge (%s->%s) has unresolvable endpoint", e.From, e.To)
			}
			var found *types.Edge
			for _, cand := range s.GetEdgesFrom(from) {
				if cand.To == to && cand.Kind == e.Kind {
					found = cand
					break
				}
			}
			if found != nil {
				if err := updateEdgeSrc(s, found.ID, mergeProps(found.Props, e.Props), ic.Source, ic.Transaction); err != nil {
					return fmt.Errorf("memory: update edge (%s->%s): %w", e.From, e.To, err)
				}
				continue
			}
			if err := addEdgeSrc(s, from, to, e.Kind, mergeProps(nil, e.Props), ic.Source, ic.Transaction); err != nil {
				return fmt.Errorf("memory: add edge (%s->%s): %w", e.From, e.To, err)
			}
		}
		return nil
	}
	if bs, ok := s.(batchStore); ok {
		return pass2Batch(bs, pass2)
	}
	return pass2()
}

type batchStore interface {
	Batch(fn func() error) error
}

func pass2Batch(bs batchStore, fn func() error) error { return bs.Batch(fn) }

// metaStore is the optional versioning write path carrying source/transaction.
type metaStore interface {
	AddNodeWithMeta(label string, props map[string]any, source, transaction string) (string, error)
	AddEdgeWithMeta(from, to, kind string, props map[string]any, source, transaction string) error
	UpdateNodeWithMeta(id string, props map[string]any, source, transaction string) error
	UpdateEdgeWithMeta(id string, props map[string]any, source, transaction string) error
}

func addNodeSrc(s storage.StorageEngine, label string, props map[string]any, source, tx string) (string, error) {
	if ms, ok := s.(metaStore); ok {
		return ms.AddNodeWithMeta(label, props, source, tx)
	}
	return s.AddNode(label, props)
}

func addEdgeSrc(s storage.StorageEngine, from, to, kind string, props map[string]any, source, tx string) error {
	if ms, ok := s.(metaStore); ok {
		return ms.AddEdgeWithMeta(from, to, kind, props, source, tx)
	}
	return s.AddEdge(from, to, kind, props)
}

func updateNodeSrc(s storage.StorageEngine, id string, props map[string]any, source, tx string) error {
	if ms, ok := s.(metaStore); ok {
		return ms.UpdateNodeWithMeta(id, props, source, tx)
	}
	return s.UpdateNode(id, props)
}

func updateEdgeSrc(s storage.StorageEngine, id string, props map[string]any, source, tx string) error {
	if ms, ok := s.(metaStore); ok {
		return ms.UpdateEdgeWithMeta(id, props, source, tx)
	}
	return s.UpdateEdge(id, props)
}

// resolveGraphID maps a memory_id to its graph ID via the pass-2 preload,
// falling back to a full scan for pre-existing nodes.
func resolveGraphID(s storage.StorageEngine, known map[string]*types.Node, memoryID string) string {
	if n, ok := known[memoryID]; ok {
		return n.ID
	}
	return findGraphID(s, memoryID)
}

// findGraphID scans all nodes for memoryID and returns its graph ID, or "".
func findGraphID(s storage.StorageEngine, memoryID string) string {
	for _, n := range s.GetAllNodes() {
		if v, _ := n.Props[PropMemoryID].(string); v == memoryID {
			return n.ID
		}
	}
	return ""
}

// mergeProps overlays new on old without mutating either input.
func mergeProps(old, new map[string]any) map[string]any {
	out := make(map[string]any, len(old)+len(new))
	for k, v := range old {
		out[k] = v
	}
	for k, v := range new {
		out[k] = v
	}
	return out
}
