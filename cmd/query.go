package cmd

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aprksy/knitknot/pkg/graph"
	"github.com/aprksy/knitknot/pkg/storage/inmem"
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
	format string
}

func init() {
	queryCmd.Flags().StringVar(&queryFlags.format, "format", "json", "Output format (json, text)")
	RootCmd.AddCommand(queryCmd)
}

// func runQuery(cmd *cobra.Command, args []string) error {
// 	// queryStr := args[0]
// 	storage := inmem.New()
// 	engine := graph.NewGraphEngine(storage)

// 	ctx := context.Background()
// 	result, err := engine.Find("User").
// 		Has("has_skill", "Go").
// 		Where("n.age", ">", 32).
// 		Exec(ctx)

// 	if err != nil {
// 		return err
// 	}

// 	switch queryFlags.format {
// 	case "text":
// 		for _, row := range result.Items() {
// 			for k, n := range row {
// 				fmt.Printf("%s: %s (%s)\n", k, n.Props["name"], n.Label)
// 			}
// 		}
// 	case "json":
// 		data, _ := json.MarshalIndent(result, "", "  ")
// 		fmt.Println(string(data))
// 	}

// 	return nil
// }

// In cmd/query.go

func runQuery(cmd *cobra.Command, args []string) error {
	dslText := args[0]
	storage := inmem.New()
	engine := graph.NewGraphEngine(storage)
	seedSampleData(engine)

	parser := dsl.NewParser(dslText)
	ast, err := parser.Parse()
	if err != nil {
		return fmt.Errorf("parse error: %w", err)
	}

	builder, err := applyAST(engine, ast)
	if err != nil {
		return fmt.Errorf("exec error: %w", err)
	}

	ctx := context.Background()
	result, err := builder.Exec(ctx)
	if err != nil {
		return err
	}

	// Output...
	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(data))

	return nil
}

func applyAST(engine *graph.GraphEngine, q *dsl.Query) (*graph.Builder, error) {
	var builder *graph.Builder

	for _, method := range q.Methods {
		switch method.Name.Value {
		case "Find":
			if len(method.Arguments) != 1 {
				return nil, fmt.Errorf("Find takes 1 arg")
			}
			if str, ok := method.Arguments[0].(*dsl.StringLiteral); ok {
				builder = engine.Find(str.Value)
			} else {
				return nil, fmt.Errorf("Find requires string")
			}

		case "Has":
			if len(method.Arguments) != 2 {
				return nil, fmt.Errorf("Has takes 2 args")
			}
			rel, ok1 := method.Arguments[0].(*dsl.StringLiteral)
			val, ok2 := method.Arguments[1].(*dsl.StringLiteral)
			if !ok1 || !ok2 {
				return nil, fmt.Errorf("Has requires two strings")
			}
			if builder != nil {
				builder = builder.Has(rel.Value, val.Value)
			}

		case "Where":
			if len(method.Arguments) != 3 {
				return nil, fmt.Errorf("Where takes 3 args")
			}
			field, ok1 := method.Arguments[0].(*dsl.StringLiteral)
			op, ok2 := method.Arguments[1].(*dsl.StringLiteral)
			var value any
			if str, ok := method.Arguments[2].(*dsl.StringLiteral); ok {
				value = str.Value
			} else if num, ok := method.Arguments[2].(*dsl.NumberLiteral); ok {
				value = num.Value
			} else {
				return nil, fmt.Errorf("Where value must be string or number")
			}
			if !ok1 || !ok2 {
				return nil, fmt.Errorf("Where field and op must be strings")
			}
			if builder != nil {
				builder = builder.Where(field.Value, op.Value, value)
			}

		case "Limit":
			if len(method.Arguments) != 1 {
				return nil, fmt.Errorf("Limit takes 1 arg")
			}
			if num, ok := method.Arguments[0].(*dsl.NumberLiteral); ok && builder != nil {
				builder = builder.Limit(num.Value)
			} else {
				return nil, fmt.Errorf("Limit requires number")
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

func seedSampleData(engine *graph.GraphEngine) {
	aliceID, _ := engine.AddNode("User", map[string]any{"name": "Alice", "age": 35})
	bobID, _ := engine.AddNode("User", map[string]any{"name": "Bob", "age": 30})
	goID, _ := engine.AddNode("Skill", map[string]any{"name": "Go"})
	_ = engine.AddEdge(aliceID, goID, "has_skill", nil)
	_ = engine.AddEdge(bobID, goID, "has_skill", map[string]any{"level": 4})
}
