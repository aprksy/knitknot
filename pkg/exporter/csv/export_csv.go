// Package csv exports a graph as two RFC 4180 tables: nodes and edges.
// Headers are id,label,<sorted prop union> and id,from,to,kind,<sorted
// prop union>; rows sort by id; missing props are empty. encoding/csv
// handles quoting; non-scalar props are JSON-encoded. Deterministic:
// same graph, same bytes (stable diffs, golden-testable).
package csv

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/aprksy/knitknot/pkg/ports/types"
)

// ExportNodes writes the node table.
func ExportNodes(nodes []*types.Node, w io.Writer) error {
	return writeTable(w, nodeColumns(nodes), nodeRows(nodes))
}

// ExportEdges writes the edge table.
func ExportEdges(edges []*types.Edge, w io.Writer) error {
	return writeTable(w, edgeColumns(edges), edgeRows(edges))
}

func nodeColumns(nodes []*types.Node) []string {
	cols := []string{"id", "label"}
	return append(cols, propUnion(nodeProps(nodes))...)
}

func edgeColumns(edges []*types.Edge) []string {
	cols := []string{"id", "from", "to", "kind"}
	return append(cols, propUnion(edgeProps(edges))...)
}

func nodeProps(nodes []*types.Node) []map[string]any {
	out := make([]map[string]any, len(nodes))
	for i, n := range nodes {
		out[i] = n.Props
	}
	return out
}

func edgeProps(edges []*types.Edge) []map[string]any {
	out := make([]map[string]any, len(edges))
	for i, e := range edges {
		out[i] = e.Props
	}
	return out
}

// propUnion returns the sorted union of property keys.
func propUnion(maps []map[string]any) []string {
	seen := map[string]bool{}
	for _, m := range maps {
		for k := range m {
			seen[k] = true
		}
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func nodeRows(nodes []*types.Node) [][]string {
	sorted := append([]*types.Node(nil), nodes...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })
	cols := nodeColumns(nodes)[2:]
	rows := make([][]string, 0, len(sorted))
	for _, n := range sorted {
		row := []string{n.ID, n.Label}
		for _, c := range cols {
			row = append(row, cell(n.Props[c]))
		}
		rows = append(rows, row)
	}
	return rows
}

func edgeRows(edges []*types.Edge) [][]string {
	sorted := append([]*types.Edge(nil), edges...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })
	cols := edgeColumns(edges)[4:]
	rows := make([][]string, 0, len(sorted))
	for _, e := range sorted {
		row := []string{e.ID, e.From, e.To, e.Kind}
		for _, c := range cols {
			row = append(row, cell(e.Props[c]))
		}
		rows = append(rows, row)
	}
	return rows
}

// cell renders one value: scalars plain, non-scalars JSON, missing empty.
func cell(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case bool, int, int64, float64:
		return fmt.Sprintf("%v", t)
	default:
		b, err := json.Marshal(t)
		if err != nil {
			return fmt.Sprintf("%v", t)
		}
		return string(b)
	}
}

func writeTable(w io.Writer, header []string, rows [][]string) error {
	cw := csv.NewWriter(w)
	if err := cw.Write(header); err != nil {
		return err
	}
	for _, r := range rows {
		if err := cw.Write(r); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}
