// pkg/graph/apply.go — AST → Builder compilation, moved from cmd/query.go
// (ADR 0009 W2): execution belongs to the kernel, cmd stays glue. Language
// knowledge lives in pkg/dsl; this file only maps it onto Builder.

package graph

import (
	"fmt"
	"strings"

	"github.com/aprksy/knitknot/pkg/dsl"
	"github.com/aprksy/knitknot/pkg/ports/query"
)

// ApplyAST compiles a parsed query AST onto the engine's fluent Builder.
func ApplyAST(engine *GraphEngine, q *dsl.Query) (*Builder, error) {
	var builder *Builder

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
