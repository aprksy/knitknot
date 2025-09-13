// cmd/repl_exec.go
package cmd

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/aprksy/knitknot/pkg/dsl"
	"github.com/aprksy/knitknot/pkg/graph"
)

func execQuery(ctx context.Context, queryStr string, engine *graph.GraphEngine, out io.Writer) error {
	parser := dsl.NewParser(queryStr)
	ast, err := parser.Parse()
	if err != nil {
		return fmt.Errorf("parse error: %w", err)
	}

	builder, err := applyAST(engine, ast)
	if err != nil {
		return fmt.Errorf("build error: %w", err)
	}

	result, err := builder.Exec(ctx)
	if err != nil {
		return err
	}

	// Pretty-print result
	if result.Empty() {
		fmt.Fprintln(out, "(no results)")
		return nil
	}

	for _, row := range result.Items() {
		var parts []string
		for varName, node := range row {
			name, _ := node.Props["name"].(string)
			if name == "" {
				name = "<unknown>"
			}
			parts = append(parts, fmt.Sprintf("%s=%s(%s)", varName, name, node.Label))
		}
		fmt.Fprintln(out, strings.Join(parts, ", "))
	}

	fmt.Fprintf(out, "-- %d result(s)\n", result.Len())
	return nil
}

func execExplain(ctx context.Context, queryStr string, engine *graph.GraphEngine, out io.Writer) error {
	queryStr = strings.TrimSpace(queryStr)
	if queryStr == "" {
		return fmt.Errorf("missing query after EXPLAIN")
	}

	parser := dsl.NewParser(queryStr)
	ast, err := parser.Parse()
	if err != nil {
		return fmt.Errorf("parse error: %w", err)
	}

	// Reuse the explain logic from before
	printExplain(queryStr, ast)

	return nil
}
