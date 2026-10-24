// Package postgres exports a graph as one Postgres script: two tables
// plus one INSERT per row inside a transaction. Tables are wide (one
// column per property, like the CSV exporter) so generated data is
// queryable with plain SQL; column types are inferred deterministically
// (all-int → INTEGER, int/float mix → DOUBLE PRECISION, all-bool →
// BOOLEAN, else TEXT; missing values are NULL). Identifiers are
// double-quoted, strings ”-escaped. Deterministic: same graph, same
// bytes (stable diffs, golden-testable).
package postgres

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/aprksy/knitknot/pkg/ports/types"
)

// ExportToPostgres writes the whole script.
func ExportToPostgres(nodes []*types.Node, edges []*types.Edge, w io.Writer) error {
	nc, ec := nodeColumns(nodes), edgeColumns(edges)
	parts := []string{"BEGIN;"}
	parts = append(parts, createTable("nodes", append([]col{{"id", "TEXT"}, {"label", "TEXT"}}, nc...)))
	parts = append(parts, createTable("edges", append([]col{{"id", "TEXT"}, {"from_id", "TEXT"}, {"to_id", "TEXT"}, {"kind", "TEXT"}}, ec...)))
	for _, r := range nodeRows(nodes, nc) {
		parts = append(parts, r)
	}
	for _, r := range edgeRows(edges, ec) {
		parts = append(parts, r)
	}
	parts = append(parts, "COMMIT;")
	_, err := fmt.Fprintln(w, strings.Join(parts, "\n"))
	return err
}

type col struct {
	name string
	typ  string
}

func createTable(name string, cols []col) string {
	defs := make([]string, len(cols))
	for i, c := range cols {
		defs[i] = fmt.Sprintf("%s %s", quoteIdent(c.name), c.typ)
	}
	return fmt.Sprintf("CREATE TABLE %s (%s);", quoteIdent(name), strings.Join(defs, ", "))
}

// propColumns returns sorted (name, inferred-type) pairs.
func propColumns(maps []map[string]any) []col {
	keys := map[string]bool{}
	vals := map[string][]any{}
	for _, m := range maps {
		for k, v := range m {
			keys[k] = true
			if v != nil {
				vals[k] = append(vals[k], v)
			}
		}
	}
	names := make([]string, 0, len(keys))
	for k := range keys {
		names = append(names, k)
	}
	sort.Strings(names)
	cols := make([]col, 0, len(names))
	for _, n := range names {
		cols = append(cols, col{n, inferType(vals[n])})
	}
	return cols
}

// inferType picks the narrowest type holding every observed value.
func inferType(vals []any) string {
	if len(vals) == 0 {
		return "TEXT"
	}
	allBool, allInt, allNum := true, true, true
	for _, v := range vals {
		switch v.(type) {
		case bool:
		default:
			allBool = false
		}
		switch v.(type) {
		case int, int64:
		default:
			allInt = false
		}
		switch v.(type) {
		case int, int64, float64:
		default:
			allNum = false
		}
	}
	switch {
	case allBool:
		return "BOOLEAN"
	case allInt:
		return "INTEGER"
	case allNum:
		return "DOUBLE PRECISION"
	default:
		return "TEXT"
	}
}

func nodeColumns(nodes []*types.Node) []col {
	maps := make([]map[string]any, len(nodes))
	for i, n := range nodes {
		maps[i] = n.Props
	}
	return propColumns(maps)
}

func edgeColumns(edges []*types.Edge) []col {
	maps := make([]map[string]any, len(edges))
	for i, e := range edges {
		maps[i] = e.Props
	}
	return propColumns(maps)
}

// lit renders one value: NULL, TRUE/FALSE, numbers plain, strings quoted.
func lit(v any) string {
	switch t := v.(type) {
	case nil:
		return "NULL"
	case string:
		return "'" + strings.ReplaceAll(t, "'", "''") + "'"
	case bool:
		if t {
			return "TRUE"
		}
		return "FALSE"
	case int, int64, float64:
		return fmt.Sprintf("%v", t)
	default:
		return "'" + strings.ReplaceAll(fmt.Sprintf("%v", t), "'", "''") + "'"
	}
}

func nodeRows(nodes []*types.Node, cols []col) []string {
	sorted := append([]*types.Node(nil), nodes...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })
	names := quoteAll(append([]string{"id", "label"}, propNames(cols)...))
	rows := make([]string, 0, len(sorted))
	for _, n := range sorted {
		vals := []string{lit(n.ID), lit(n.Label)}
		for _, c := range cols {
			vals = append(vals, lit(n.Props[c.name]))
		}
		rows = append(rows, insertLit("nodes", names, vals))
	}
	return rows
}

func edgeRows(edges []*types.Edge, cols []col) []string {
	sorted := append([]*types.Edge(nil), edges...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })
	names := quoteAll(append([]string{"id", "from_id", "to_id", "kind"}, propNames(cols)...))
	rows := make([]string, 0, len(sorted))
	for _, e := range sorted {
		vals := []string{lit(e.ID), lit(e.From), lit(e.To), lit(e.Kind)}
		for _, c := range cols {
			vals = append(vals, lit(e.Props[c.name]))
		}
		rows = append(rows, insertLit("edges", names, vals))
	}
	return rows
}

func propNames(cols []col) []string {
	names := make([]string, len(cols))
	for i, c := range cols {
		names[i] = c.name
	}
	return names
}

func quoteAll(names []string) []string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = quoteIdent(n)
	}
	return out
}

func insertLit(table string, cols, vals []string) string {
	return fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s);", quoteIdent(table), strings.Join(cols, ", "), strings.Join(vals, ", "))
}

func quoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}
