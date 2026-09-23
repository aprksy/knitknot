package dot

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/aprksy/knitknot/pkg/ports/types"
)

// exp: dot-hardening — DOT C-style quoting: backslash, quote, newline.
func dotQuote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// exp: dot-hardening — node IDs are DOT-quoted identifiers so spaces,
// dots, quotes, leading digits, and non-ASCII can't break the syntax.
func exportNodeName(id string) string {
	return dotQuote(fmt.Sprintf("N_%s", strings.ReplaceAll(id, "-", "_")))
}

// ExportToDOT writes the graph in DOT format
func ExportToDOT(nodes []*types.Node, edges []*types.Edge, w io.Writer) error {
	_, err := fmt.Fprintf(w, "digraph KnitKnot {\n")
	if err != nil {
		return err
	}

	// exp: dot-hardening — sorted output for stable diffs.
	sortedNodes := make([]*types.Node, len(nodes))
	copy(sortedNodes, nodes)
	sort.Slice(sortedNodes, func(i, j int) bool { return sortedNodes[i].ID < sortedNodes[j].ID })

	// Nodes
	for _, n := range sortedNodes {
		label := n.Label
		// exp: dot-hardening — empty name falls through to title.
		if name, ok := n.Props["name"]; ok && name != "" {
			label = fmt.Sprintf("%s:%v", n.Label, name)
		} else if title, ok := n.Props["title"]; ok {
			label = fmt.Sprintf("%s:%v", n.Label, title)
		}
		_, err := fmt.Fprintf(w, "  %s [label=%s, shape=box, style=rounded];\n",
			exportNodeName(n.ID), dotQuote(label))
		if err != nil {
			return err
		}
	}

	// exp: dot-hardening — sorted output for stable diffs.
	sortedEdges := make([]*types.Edge, len(edges))
	copy(sortedEdges, edges)
	sort.Slice(sortedEdges, func(i, j int) bool {
		if sortedEdges[i].ID != sortedEdges[j].ID {
			return sortedEdges[i].ID < sortedEdges[j].ID
		}
		if sortedEdges[i].From != sortedEdges[j].From {
			return sortedEdges[i].From < sortedEdges[j].From
		}
		if sortedEdges[i].To != sortedEdges[j].To {
			return sortedEdges[i].To < sortedEdges[j].To
		}
		return sortedEdges[i].Kind < sortedEdges[j].Kind
	})

	// Edges
	seen := make(map[string]bool)
	for _, edge := range sortedEdges {
		fromName := exportNodeName(edge.From)
		toName := exportNodeName(edge.To)
		// exp: dot-hardening — dedup on mapped names (so DOT-level
		// collisions collapse) plus the storage-assigned edge ID (so
		// distinct edges sharing from->to@kind both emit).
		key := fromName + "->" + toName + "@" + edge.Kind + "\x00" + edge.ID
		if seen[key] {
			continue
		}
		seen[key] = true

		label := dotQuote(edge.Kind)

		_, err := fmt.Fprintf(w, "  %s -> %s [label=%s];\n", fromName, toName, label)
		if err != nil {
			return err
		}
	}

	_, err = fmt.Fprintf(w, "}\n")
	return err
}
