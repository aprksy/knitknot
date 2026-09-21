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
	var results []map[string]*types.Node

	// Start with first node pattern
	if len(plan.Nodes) == 0 {
		return &ResultSet{items: results}, nil
	}

	first := plan.Nodes[0]
	var candidates []*types.Node // perf: index
	if plan.Subgraph != "" {     // fix: H3
		candidates = filterNodesByLabel(storage.GetNodesIn(plan.Subgraph), first.Label) // fix: H3
	} else { // fix: H3
		candidates = storage.GetNodesByLabel(first.Label) // perf: index
	}

	for _, node := range candidates {
		row := map[string]*types.Node{
			first.Var: node,
		}

		results = append(results, row)
	}

	// Now extend with remaining nodes + edges
	for _, edgePattern := range plan.Edges {
		results = qe.expandViaEdge(storage, results, edgePattern, plan.Nodes, plan.Filters)
	}

	// Apply final filters (some may involve multiple vars)
	filtered := qe.applyAllFilters(results, first.Var, plan.Filters)

	// Apply limit
	if plan.LimitVal != nil && len(filtered) > *plan.LimitVal {
		filtered = filtered[:*plan.LimitVal]
	}

	return NewResultSet(filtered), nil
}

func (qe *DefaultQueryEngine) matchFilters(row map[string]*types.Node, primaryVar string, filters []query.Filter) bool {
	for _, f := range filters {
		// Extract var name: e.g., "n.age" → var="n", prop="age"
		var varName, prop string
		parts := strings.SplitN(f.Field, ".", 2)
		if len(parts) != 2 {
			varName, prop = primaryVar, f.Field
		} else {
			varName, prop = parts[0], parts[1]
		}

		node, ok := row[varName]
		if !ok {
			return false
		}

		val, ok := node.Props[prop]
		if !ok {
			return false
		}

		if !compare(val, f.Op, f.Value) {
			return false
		}
	}
	return true
}

func (qe *DefaultQueryEngine) applyAllFilters(rows []map[string]*types.Node, primaryVar string, filters []query.Filter) []map[string]*types.Node {
	var result []map[string]*types.Node
	for _, row := range rows {
		if qe.matchFilters(row, primaryVar, filters) {
			result = append(result, row)
		}
	}

	return result
}

func (qe *DefaultQueryEngine) findLabelForVar(varName string, nodes []*query.PatternNode) string {
	for _, n := range nodes {
		if n.Var == varName {
			return n.Label
		}
	}
	return ""
}

func (qe *DefaultQueryEngine) expandViaEdge(
	storage storage.StorageEngine,
	rows []map[string]*types.Node,
	edgePattern *query.PatternEdge,
	allNodes []*query.PatternNode,
	filters []query.Filter,
) []map[string]*types.Node {
	var expanded []map[string]*types.Node

	fromVar := edgePattern.From
	toVar := edgePattern.To
	kind := edgePattern.Kind

	for _, row := range rows {
		fromNode, ok := row[fromVar]
		if !ok {
			continue
		}

		// Get ALL outgoing edges of this kind
		for _, e := range storage.GetEdgesFrom(fromNode.ID) {
			if e.Kind != kind {
				continue
			}

			// Check edge filters BEFORE accepting
			if !qe.matchEdgeFilters(e, edgePattern.Filters) {
				continue
			}

			toNode, ok := storage.GetNode(e.To)
			if !ok {
				continue
			}

			expectedLabel := qe.findLabelForVar(toVar, allNodes)
			if expectedLabel != "" && toNode.Label != expectedLabel {
				continue
			}

			newRow := copyMap(row)
			// newRow := map[string]*types.Node{}
			newRow[toVar] = toNode

			// Node/prop filters applied later
			expanded = append(expanded, newRow)
		}
	}

	return expanded
}

func (qe *DefaultQueryEngine) matchEdgeFilters(edge *types.Edge, filters []query.Filter) bool {
	for _, f := range filters {
		val, ok := edge.Props[f.Field]
		if !ok {
			return false
		}
		if !compare(val, f.Op, f.Value) {
			return false
		}
	}
	return true
}

func copyMap(m map[string]*types.Node) map[string]*types.Node {
	if m == nil {
		return nil
	}
	cp := make(map[string]*types.Node, len(m))
	for k, v := range m {
		cp[k] = v
	}
	return cp
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
	case "=", "!=":
		if af, ok := toFloat(a); ok {
			if bf, ok := toFloat(b); ok {
				if op == "=" {
					return af == bf
				}
				return af != bf
			}
		}
		if op == "=" {
			return a == b
		}
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
