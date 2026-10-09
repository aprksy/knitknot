package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aprksy/knitknot/pkg/graph"
	"github.com/spf13/cobra"

	"github.com/aprksy/knitknot/pkg/dsl"
	"github.com/aprksy/knitknot/pkg/ports/query"
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
	// fix: H1 debug line removed (was fmt.Fprintf(os.Stderr, "INPUT: ..."))

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
	if engine, err = resolveStoreBackend(engine); err != nil { // store: wire
		return err // store: wire
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
			if len(method.Arguments) < 1 { // fix: H5
				fmt.Printf("    %d. Find (unrecognized arg)\n", i+1) // fix: H5
				continue                                             // fix: H5
			} // fix: H5
			if arg, ok := method.Arguments[0].(*dsl.StringLiteral); ok { // fix: H5
				fmt.Printf("    %d. Match nodes with label '%s'\n", i+1, arg.Value)
			} else { // fix: H5
				fmt.Printf("    %d. Match nodes with label '???'\n", i+1) // fix: H5
			}
		case "Has":
			if len(method.Arguments) < 2 { // fix: H5
				fmt.Printf("    %d. Has (unrecognized arg)\n", i+1) // fix: H5
				continue                                            // fix: H5
			} // fix: H5
			rel, ok1 := method.Arguments[0].(*dsl.StringLiteral) // fix: H5
			val, ok2 := method.Arguments[1].(*dsl.StringLiteral) // fix: H5
			if !ok1 || !ok2 {                                    // fix: H5
				fmt.Printf("    %d. Has (unrecognized arg)\n", i+1) // fix: H5
				continue                                            // fix: H5
			} // fix: H5
			fmt.Printf("    %d. Follow '%s' edges to nodes with value '%s'\n", i+1, rel.Value, val.Value)
		case "Follow":
			if len(method.Arguments) < 1 || len(method.Arguments) > 2 {
				fmt.Printf("    %d. Follow (unrecognized arg)\n", i+1)
				continue
			}
			rel, ok := method.Arguments[0].(*dsl.StringLiteral)
			if !ok {
				fmt.Printf("    %d. Follow (unrecognized arg)\n", i+1)
				continue
			}
			fmt.Printf("    %d. Follow '%s' edges (%s)\n", i+1, rel.Value, explainDir(method.Arguments))
		case "FollowHas":
			if len(method.Arguments) < 2 || len(method.Arguments) > 3 {
				fmt.Printf("    %d. FollowHas (unrecognized arg)\n", i+1)
				continue
			}
			rel, ok1 := method.Arguments[0].(*dsl.StringLiteral)
			val, ok2 := method.Arguments[1].(*dsl.StringLiteral)
			if !ok1 || !ok2 {
				fmt.Printf("    %d. FollowHas (unrecognized arg)\n", i+1)
				continue
			}
			fmt.Printf("    %d. Follow '%s' edges to nodes with value '%s' (%s)\n", i+1, rel.Value, val.Value, explainDir(method.Arguments[1:]))
		case "Reach":
			if len(method.Arguments) < 1 || len(method.Arguments) > 3 {
				fmt.Printf("    %d. Reach (unrecognized arg)\n", i+1)
				continue
			}
			rel, ok := method.Arguments[0].(*dsl.StringLiteral)
			if !ok {
				fmt.Printf("    %d. Reach (unrecognized arg)\n", i+1)
				continue
			}
			dir, depth := explainReachOpts(method.Arguments[1:])
			fmt.Printf("    %d. Reach up to %s hops along '%s' edges (%s)\n", i+1, depth, rel.Value, dir)
		case "ReachHas":
			if len(method.Arguments) < 2 || len(method.Arguments) > 4 {
				fmt.Printf("    %d. ReachHas (unrecognized arg)\n", i+1)
				continue
			}
			rel, ok1 := method.Arguments[0].(*dsl.StringLiteral)
			val, ok2 := method.Arguments[1].(*dsl.StringLiteral)
			if !ok1 || !ok2 {
				fmt.Printf("    %d. ReachHas (unrecognized arg)\n", i+1)
				continue
			}
			dir, depth := explainReachOpts(method.Arguments[2:])
			fmt.Printf("    %d. Reach up to %s hops along '%s' edges to nodes with value '%s' (%s)\n", i+1, depth, rel.Value, val.Value, dir)
		case "Where", "WhereEdge":
			if len(method.Arguments) < 3 { // fix: H5
				fmt.Printf("    %d. %s (unrecognized arg)\n", i+1, method.Name.Value) // fix: H5
				continue                                                              // fix: H5
			} // fix: H5
			field, ok1 := method.Arguments[0].(*dsl.StringLiteral) // fix: H5
			op, ok2 := method.Arguments[1].(*dsl.StringLiteral)    // fix: H5
			if !ok1 || !ok2 {                                      // fix: H5
				fmt.Printf("    %d. %s (unrecognized arg)\n", i+1, method.Name.Value) // fix: H5
				continue                                                              // fix: H5
			} // fix: H5
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
			if len(method.Arguments) < 1 { // fix: H5
				fmt.Printf("    %d. Limit (unrecognized arg)\n", i+1) // fix: H5
				continue                                              // fix: H5
			} // fix: H5
			if n, ok := method.Arguments[0].(*dsl.NumberLiteral); ok { // fix: H5
				fmt.Printf("    %d. Limit result to %d items\n", i+1, n.Value)
			} else { // fix: H5
				fmt.Printf("    %d. Limit (unrecognized arg)\n", i+1) // fix: H5
			}
		default:
			fmt.Printf("    %d. Unknown operation: %s\n", i+1, method.Name.Value)
		}
	}
}

func explainDir(args []dsl.Expression) string {
	if len(args) == 0 {
		return "out"
	}
	last := args[len(args)-1]
	if s, ok := last.(*dsl.StringLiteral); ok {
		switch strings.ToLower(s.Value) {
		case "in", "out", "both":
			return strings.ToLower(s.Value)
		}
	}
	return "out"
}

func parseDirection(s string) (query.Direction, error) {
	switch strings.ToLower(s) {
	case "out":
		return query.Out, nil
	case "in":
		return query.In, nil
	case "both":
		return query.Both, nil
	default:
		return query.Out, fmt.Errorf("direction must be 'out', 'in' or 'both', got %q", s)
	}
}

// parseReachOpts parses the optional trailing [direction, depth] args of
// Reach/ReachHas. Depth omitted ⇒ empty slice (builder default applies).
func parseReachOpts(args []dsl.Expression) (query.Direction, []int, error) {
	dir := query.Out
	var depth []int
	for _, a := range args {
		switch v := a.(type) {
		case *dsl.StringLiteral:
			d, err := parseDirection(v.Value)
			if err != nil {
				return dir, nil, err
			}
			dir = d
		case *dsl.NumberLiteral:
			depth = []int{v.Value}
		default:
			return dir, nil, fmt.Errorf("reach options must be a direction string and a depth number")
		}
	}
	return dir, depth, nil
}

func explainReachOpts(args []dsl.Expression) (dir, depth string) {
	dir, depth = "out", "default"
	for _, a := range args {
		switch v := a.(type) {
		case *dsl.StringLiteral:
			switch strings.ToLower(v.Value) {
			case "in", "out", "both":
				dir = strings.ToLower(v.Value)
			}
		case *dsl.NumberLiteral:
			depth = fmt.Sprintf("%d", v.Value)
		}
	}
	return dir, depth
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

		case "Follow":
			if len(method.Arguments) < 1 || len(method.Arguments) > 2 {
				return nil, fmt.Errorf("follow takes 1 or 2 args: rel and optional direction")
			}
			rel, ok := method.Arguments[0].(*dsl.StringLiteral)
			if !ok {
				return nil, fmt.Errorf("follow requires rel string")
			}
			dir := query.Out
			if len(method.Arguments) == 2 {
				dirStr, ok := method.Arguments[1].(*dsl.StringLiteral)
				if !ok {
					return nil, fmt.Errorf("follow direction must be a string")
				}
				var err error
				if dir, err = parseDirection(dirStr.Value); err != nil {
					return nil, err
				}
			}
			if builder != nil {
				builder = builder.Follow(rel.Value, dir)
			}

		case "FollowHas":
			if len(method.Arguments) < 2 || len(method.Arguments) > 3 {
				return nil, fmt.Errorf("followhas takes 2 or 3 args: rel, value and optional direction")
			}
			rel, ok1 := method.Arguments[0].(*dsl.StringLiteral)
			val, ok2 := method.Arguments[1].(*dsl.StringLiteral)
			if !ok1 || !ok2 {
				return nil, fmt.Errorf("followhas requires rel and value strings")
			}
			dir := query.Out
			if len(method.Arguments) == 3 {
				dirStr, ok := method.Arguments[2].(*dsl.StringLiteral)
				if !ok {
					return nil, fmt.Errorf("followhas direction must be a string")
				}
				var err error
				if dir, err = parseDirection(dirStr.Value); err != nil {
					return nil, err
				}
			}
			if builder != nil {
				builder = builder.FollowHas(rel.Value, val.Value, dir)
			}

		case "Reach":
			if len(method.Arguments) < 1 || len(method.Arguments) > 3 {
				return nil, fmt.Errorf("reach takes 1 to 3 args: rel and optional direction and depth")
			}
			rel, ok := method.Arguments[0].(*dsl.StringLiteral)
			if !ok {
				return nil, fmt.Errorf("reach requires rel string")
			}
			dir, depth, err := parseReachOpts(method.Arguments[1:])
			if err != nil {
				return nil, err
			}
			if builder != nil {
				builder = builder.Reach(rel.Value, dir, depth...)
			}

		case "ReachHas":
			if len(method.Arguments) < 2 || len(method.Arguments) > 4 {
				return nil, fmt.Errorf("reachhas takes 2 to 4 args: rel, value and optional direction and depth")
			}
			rel, ok1 := method.Arguments[0].(*dsl.StringLiteral)
			val, ok2 := method.Arguments[1].(*dsl.StringLiteral)
			if !ok1 || !ok2 {
				return nil, fmt.Errorf("reachhas requires rel and value strings")
			}
			dir, depth, err := parseReachOpts(method.Arguments[2:])
			if err != nil {
				return nil, err
			}
			if builder != nil {
				builder = builder.ReachHas(rel.Value, val.Value, dir, depth...)
			}

		case "In": // fix: H3
			if len(method.Arguments) != 1 {
				return nil, fmt.Errorf("in takes 1 arg")
			}
			if str, ok := method.Arguments[0].(*dsl.StringLiteral); ok && builder != nil {
				builder = builder.In(str.Value) // fix: H3
			} else {
				return nil, fmt.Errorf("in requires string")
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
