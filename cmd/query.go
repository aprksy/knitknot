package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/aprksy/knitknot/pkg/graph"
	"github.com/spf13/cobra"

	"github.com/aprksy/knitknot/pkg/dsl"
)

var queryCmd = &cobra.Command{
	Use:   "query",
	Short: "Run a KnitKnot query",
	Args:  cobra.ExactArgs(1),
	RunE:  runQuery,
}

var queryFlags struct {
	format  string
	dryRun  bool
	explain bool
}

func init() {
	queryCmd.Flags().StringVar(&globalFlags.subgraph, "subgraph", "", "Run query within a subgraph context")
	queryCmd.Flags().StringVar(&queryFlags.format, "format", "text", "Output format (json, text)")
	queryCmd.Flags().BoolVar(&queryFlags.dryRun, "dry-run", false, "Parse and validate query, but don't execute")
	queryCmd.Flags().BoolVar(&queryFlags.explain, "explain", false, "Show query execution plan")
	RootCmd.AddCommand(queryCmd)
}

func runQuery(cmd *cobra.Command, args []string) error {
	dslText := args[0]
	fmt.Fprintf(os.Stderr, "INPUT: %q\n", dslText)

	// Parse first
	parser := dsl.NewParser(dslText)
	ast, err := parser.Parse()
	if err != nil {
		return fmt.Errorf("parse error: %w", err)
	}

	// Show plan if --explain
	if queryFlags.explain {
		printExplain(dslText, ast)
	}

	// Exit early if --dry-run
	if queryFlags.dryRun {
		if !queryFlags.explain {
			fmt.Println("Syntax OK")
		}
		return nil
	}

	// Load graph based on -f flag
	engine, err := LoadGraph(globalFlags.file)
	if err != nil {
		return err
	}

	// if subgraph specified
	if globalFlags.subgraph != "" {
		engine = engine.WithSubgraph(globalFlags.subgraph)
	}

	builder, err := ApplyAST(engine, ast)
	if err != nil {
		return fmt.Errorf("exec error: %w", err)
	}

	ctx := context.Background()
	result, err := builder.Exec(ctx)
	if err != nil {
		return err
	}

	// Output result
	switch queryFlags.format {
	case "text":
		fmt.Println("RESULT (text):")
		for _, row := range result.Items() {
			index := 0
			for k, n := range row {
				name := n.Props["name"]
				if name == nil {
					name = "?"
				}
				prefix := "    "
				if index == 0 {
					prefix = "  - "
				}
				fmt.Printf("%s%s: %v (%s)\n", prefix, k, name, n.Label)
				index++
			}
		}
	case "json":
		fmt.Println("RESULT (json):")
		data, err := json.Marshal(result)
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	return nil
}

func printExplain(queryStr string, ast *dsl.Query) {
	fmt.Println("Query Plan:")
	fmt.Printf("  Raw Query: %s\n", queryStr)
	fmt.Println("  Steps:")
	for i, method := range ast.Methods {
		switch method.Name.Value {
		case "Find":
			arg := method.Arguments[0].(*dsl.StringLiteral)
			fmt.Printf("    %d. Match nodes with label '%s'\n", i+1, arg.Value)
		case "Has":
			rel := method.Arguments[0].(*dsl.StringLiteral)
			val := method.Arguments[1].(*dsl.StringLiteral)
			fmt.Printf("    %d. Follow '%s' edges to nodes with value '%s'\n", i+1, rel.Value, val.Value)
		case "Where", "WhereEdge":
			field := method.Arguments[0].(*dsl.StringLiteral)
			op := method.Arguments[1].(*dsl.StringLiteral)
			value := method.Arguments[2]
			var valStr string
			switch v := value.(type) {
			case *dsl.StringLiteral:
				valStr = fmt.Sprintf("%q", v.Value)
			case *dsl.NumberLiteral:
				valStr = fmt.Sprintf("%v", v.Value)
			default:
				valStr = "???"
			}
			fmt.Printf("    %d. Filter where %s %s %s\n", i+1, field.Value, op.Value, valStr)
		case "Limit":
			n := method.Arguments[0].(*dsl.NumberLiteral)
			fmt.Printf("    %d. Limit result to %d items\n", i+1, n.Value)
		default:
			fmt.Printf("    %d. Unknown operation: %s\n", i+1, method.Name.Value)
		}
	}
}

func ApplyAST(engine *graph.GraphEngine, q *dsl.Query) (*graph.Builder, error) {
	var builder *graph.Builder

	for _, method := range q.Methods {
		switch method.Name.Value {
		case "Find":
			if len(method.Arguments) != 1 {
				return nil, fmt.Errorf("find takes 1 arg")
			}
			if str, ok := method.Arguments[0].(*dsl.StringLiteral); ok {
				builder = engine.Find(str.Value)
			} else {
				return nil, fmt.Errorf("find requires string")
			}

		case "Has":
			if len(method.Arguments) != 2 {
				return nil, fmt.Errorf("has takes 2 args")
			}
			rel, ok1 := method.Arguments[0].(*dsl.StringLiteral)
			val, ok2 := method.Arguments[1].(*dsl.StringLiteral)
			if !ok1 || !ok2 {
				return nil, fmt.Errorf("has requires two strings")
			}
			if builder != nil {
				builder = builder.Has(rel.Value, val.Value)
			}

		case "Where":
			if len(method.Arguments) != 3 {
				return nil, fmt.Errorf("where takes 3 args")
			}
			field, ok1 := method.Arguments[0].(*dsl.StringLiteral)
			op, ok2 := method.Arguments[1].(*dsl.StringLiteral)
			var value any
			if str, ok := method.Arguments[2].(*dsl.StringLiteral); ok {
				value = str.Value
			} else if num, ok := method.Arguments[2].(*dsl.NumberLiteral); ok {
				value = num.Value
			} else {
				return nil, fmt.Errorf("where value must be string or number")
			}
			if !ok1 || !ok2 {
				return nil, fmt.Errorf("where field and op must be strings")
			}
			if builder != nil {
				builder = builder.Where(field.Value, op.Value, value)
			}

		case "WhereEdge":
			if len(method.Arguments) != 3 {
				return nil, fmt.Errorf("where takes 3 args")
			}
			field, ok1 := method.Arguments[0].(*dsl.StringLiteral)
			op, ok2 := method.Arguments[1].(*dsl.StringLiteral)
			var value any
			if str, ok := method.Arguments[2].(*dsl.StringLiteral); ok {
				value = str.Value
			} else if num, ok := method.Arguments[2].(*dsl.NumberLiteral); ok {
				value = num.Value
			} else {
				return nil, fmt.Errorf("where value must be string or number")
			}
			if !ok1 || !ok2 {
				return nil, fmt.Errorf("where field and op must be strings")
			}
			if builder != nil {
				builder = builder.WhereEdge(field.Value, op.Value, value)
			}

		case "Limit":
			if len(method.Arguments) != 1 {
				return nil, fmt.Errorf("limit takes 1 arg")
			}
			if num, ok := method.Arguments[0].(*dsl.NumberLiteral); ok && builder != nil {
				builder = builder.Limit(num.Value)
			} else {
				return nil, fmt.Errorf("limit requires number")
			}

		default:
			return nil, fmt.Errorf("unknown method: %s", method.Name.Value)
		}
	}

	if builder == nil {
		return nil, fmt.Errorf("empty query")
	}

	return builder, nil
}
