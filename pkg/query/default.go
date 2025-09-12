package query

import (
	"context"
	"strconv"
	"strings"

	"github.com/aprksy/knitknot/pkg/ports/query"
	"github.com/aprksy/knitknot/pkg/ports/storage"
	"github.com/aprksy/knitknot/pkg/ports/types"
)

var _ query.QueryEngine = (*DefaultQueryEngine)(nil)

type DefaultQueryEngine struct{}

func NewDefaultQueryEngine() *DefaultQueryEngine {
	return &DefaultQueryEngine{}
}

func (qe *DefaultQueryEngine) Execute(
	ctx context.Context,
	storage storage.StorageEngine,
	plan *query.QueryPlan,
) (query.ResultSet, error) {

	// For now: super simple execution
	// Later: pattern matching, filtering, joins

	results := make([]map[string]*types.Node, 0)

	// If no nodes in pattern, return all?
	if len(plan.Nodes) == 0 {
		return &ResultSet{items: results}, nil
	}

	// Start with first node
	for _, nodePattern := range plan.Nodes {
		candidates := filterNodesByLabel(storage.GetAllNodes(), nodePattern.Label)
		for _, node := range candidates {
			row := map[string]*types.Node{
				nodePattern.Var: node,
			}

			// Apply filters like "u.name = 'Alice'"
			matched := true
			for _, f := range plan.Filters {
				if strings.HasPrefix(f.Field, nodePattern.Var+".") {
					prop := f.Field[len(nodePattern.Var)+1:]
					val, ok := node.Props[prop]
					if !ok {
						matched = false
						break
					}
					if !compare(val, f.Op, f.Value) {
						matched = false
						break
					}
				}
			}
			if matched {
				results = append(results, row)
			}
		}
	}

	// Apply limit
	if plan.LimitVal != nil && len(results) > *plan.LimitVal {
		results = results[:*plan.LimitVal]
	}

	return &ResultSet{items: results}, nil
}

func filterNodesByLabel(nodes []*types.Node, label string) []*types.Node {
	var filtered []*types.Node
	for _, n := range nodes {
		if n.Label == label {
			filtered = append(filtered, n)
		}
	}
	return filtered
}

func compare(a any, op string, b any) bool {
	switch op {
	case "=":
		return a == b
	case "!=":
		return a != b
	case ">":
		if ai, ok := toFloat(a); ok {
			if bi, ok := toFloat(b); ok {
				return ai > bi
			}
		}
	case "<":
		if ai, ok := toFloat(a); ok {
			if bi, ok := toFloat(b); ok {
				return ai < bi
			}
		}
	}
	return false
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case string:
		f, err := strconv.ParseFloat(n, 64)
		return f, err == nil
	default:
		return 0, false
	}
}
