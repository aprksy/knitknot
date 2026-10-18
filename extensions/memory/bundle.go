package memory

import (
	"encoding/json"
	"fmt"
	"io"
)

// Bundle is the memory-bundle v1 interchange shape (docs/domains/memory/BUNDLE.md).
type Bundle struct {
	Type    string       `json:"type"`
	Version int          `json:"version"`
	Nodes   []BundleNode `json:"nodes"`
	Edges   []BundleEdge `json:"edges"`
}

// BundleNode is one node in a memory-bundle.
type BundleNode struct {
	MemoryID string         `json:"memory_id"`
	Label    string         `json:"label"`
	Props    map[string]any `json:"props"`
}

// BundleEdge is one edge in a memory-bundle; From/To are memory_id references.
type BundleEdge struct {
	From  string         `json:"from"`
	To    string         `json:"to"`
	Kind  string         `json:"kind"`
	Props map[string]any `json:"props"`
}

// ParseBundle decodes and validates a memory-bundle v1 document.
func ParseBundle(r io.Reader) (*Bundle, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("memory: read bundle: %w", err)
	}
	var b Bundle
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, fmt.Errorf("memory: parse bundle: %w", err)
	}
	if b.Type != "" && b.Type != "memory-bundle" {
		return nil, fmt.Errorf("memory: bundle type %q, want %q", b.Type, "memory-bundle")
	}
	for i := range b.Nodes {
		n := &b.Nodes[i]
		if n.MemoryID == "" {
			return nil, fmt.Errorf("memory: node %d missing memory_id", i)
		}
		if n.Label == "" {
			return nil, fmt.Errorf("memory: node %q missing label", n.MemoryID)
		}
		if !IsSupportedLabel(n.Label) {
			return nil, fmt.Errorf("memory: unknown label %q", n.Label)
		}
		if n.Props == nil {
			n.Props = map[string]any{}
		}
	}
	for i := range b.Edges {
		e := &b.Edges[i]
		if e.From == "" || e.To == "" {
			return nil, fmt.Errorf("memory: edge %d missing from or to", i)
		}
		if e.Kind == "" {
			return nil, fmt.Errorf("memory: edge %d (%s->%s) missing kind", i, e.From, e.To)
		}
		if !IsSupportedEdgeKind(e.Kind) {
			return nil, fmt.Errorf("memory: unknown edge kind %q", e.Kind)
		}
		if e.Props == nil {
			e.Props = map[string]any{}
		}
		if e.Kind == EdgeSimilarTo {
			if _, ok := e.Props[PropScore]; !ok {
				return nil, fmt.Errorf("memory: similar-to edge (%s->%s) missing %q prop", e.From, e.To, PropScore)
			}
			if _, ok := e.Props[PropModel]; !ok {
				return nil, fmt.Errorf("memory: similar-to edge (%s->%s) missing %q prop", e.From, e.To, PropModel)
			}
		}
	}
	return &b, nil
}
