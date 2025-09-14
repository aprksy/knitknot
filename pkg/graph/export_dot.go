package graph

import (
	"fmt"
	"io"
	"strings"

	"github.com/aprksy/knitknot/pkg/ports/types"
)

func exportNodeName(id string) string {
	// Clean node ID for DOT
	return fmt.Sprintf("N_%s", strings.ReplaceAll(id, "-", "_"))
}

func LabelSafe(s string) string {
	return fmt.Sprintf("%q", s) // wrap in quotes
}

// ExportToDOT writes the graph in DOT format
func ExportToDOT(nodes []*types.Node, edges []*types.Edge, w io.Writer) error {
	_, err := fmt.Fprintf(w, "digraph KnitKnot {\n")
	if err != nil {
		return err
	}

	// Nodes
	for _, n := range nodes {
		label := n.Label
		if name, ok := n.Props["name"]; ok {
			label = fmt.Sprintf("%s:%v", n.Label, name)
		} else if title, ok := n.Props["title"]; ok {
			label = fmt.Sprintf("%s:%v", n.Label, title)
		}
		_, err := fmt.Fprintf(w, "  %s [label=%s, shape=box, style=rounded];\n",
			exportNodeName(n.ID), LabelSafe(label))
		if err != nil {
			return err
		}
	}

	// Edges
	seen := make(map[string]bool)
	for _, edge := range edges {
		key := fmt.Sprintf("%s->%s@%s", edge.From, edge.To, edge.Kind)
		if seen[key] {
			continue
		}
		seen[key] = true

		fromName := exportNodeName(edge.From)
		toName := exportNodeName(edge.To)
		label := fmt.Sprintf("%q", edge.Kind)

		_, err := fmt.Fprintf(w, "  %s -> %s [label=%s];\n", fromName, toName, label)
		if err != nil {
			return err
		}
	}

	_, err = fmt.Fprintf(w, "}\n")
	return err
}

// func getAllEdges(engine *GraphEngine) []*types.Edge {
// 	edges := make([]*types.Edge, 0)
// 	storage := engine.Storage()

// 	for _, n := range storage.GetAllNodes() {
// 		edges = append(edges, storage.GetEdgesFrom(n.ID)...)
// 	}
// 	return dedupEdges(edges)
// }

// func dedupEdges(edges []*types.Edge) []*types.Edge {
// 	seen := make(map[string]*types.Edge)
// 	for _, e := range edges {
// 		key := fmt.Sprintf("%s->%s@%s", e.From, e.To, e.Kind)
// 		seen[key] = e
// 	}
// 	result := make([]*types.Edge, 0, len(seen))
// 	for _, e := range seen {
// 		result = append(result, e)
// 	}
// 	return result
// }
