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
		results = qe.expandViaEdge(storage, results, edgePattern, plan.Nodes, plan.Filters, plan.Subgraph)
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
	subgraph string,
) []map[string]*types.Node {
	if edgePattern.MaxDepth > 1 {
		return qe.expandReachable(storage, rows, edgePattern, allNodes, subgraph)
	}

	var expanded []map[string]*types.Node

	fromVar := edgePattern.From
	toVar := edgePattern.To
	kind := edgePattern.Kind

	// NOTE: single-hop expansion intentionally ignores subgraph membership
	// (pre-existing gap — only Execute's initial candidates are filtered).
	// Subgraph restriction applies to the BFS path below.

	for _, row := range rows {
		fromNode, ok := row[fromVar]
		if !ok {
			continue
		}

		seen := map[string]bool{} // ponytail: per-row target dedup; Both union collapses here
		expand := func(edges []*types.Edge, target func(*types.Edge) string) {
			for _, e := range edges {
				if kind != "" && e.Kind != kind {
					continue
				}

				// Check edge filters BEFORE accepting
				if !qe.matchEdgeFilters(e, edgePattern.Filters) {
					continue
				}

				targetID := target(e)
				if seen[targetID] {
					continue
				}

				toNode, ok := storage.GetNode(targetID)
				if !ok {
					continue
				}

				expectedLabel := qe.findLabelForVar(toVar, allNodes)
				if expectedLabel != "" && toNode.Label != expectedLabel {
					continue
				}

				seen[targetID] = true
				newRow := copyMap(row)
				// newRow := map[string]*types.Node{}
				newRow[toVar] = toNode

				// Node/prop filters applied later
				expanded = append(expanded, newRow)
			}
		}

		switch edgePattern.Direction {
		case query.In:
			expand(storage.GetEdgesTo(fromNode.ID), func(e *types.Edge) string { return e.From })
		case query.Both:
			expand(storage.GetEdgesFrom(fromNode.ID), func(e *types.Edge) string { return e.To })
			expand(storage.GetEdgesTo(fromNode.ID), func(e *types.Edge) string { return e.From })
		default: // query.Out
			expand(storage.GetEdgesFrom(fromNode.ID), func(e *types.Edge) string { return e.To })
		}
	}

	return expanded
}

// expandReachable runs a BFS from row[From] to depth MaxDepth and emits one
// row per reached node at depths [MinDepth, MaxDepth]. The per-source visited
// set (seeded with the source) makes it cycle-safe; output is the node set,
// not paths.
func (qe *DefaultQueryEngine) expandReachable(
	storage storage.StorageEngine,
	rows []map[string]*types.Node,
	edgePattern *query.PatternEdge,
	allNodes []*query.PatternNode,
	subgraph string,
) []map[string]*types.Node {
	var expanded []map[string]*types.Node

	fromVar := edgePattern.From
	toVar := edgePattern.To
	minDepth := edgePattern.MinDepth
	if minDepth < 1 {
		minDepth = 1
	}

	var inSubgraph map[string]bool
	if subgraph != "" {
		inSubgraph = map[string]bool{}
		for _, n := range storage.GetNodesIn(subgraph) {
			inSubgraph[n.ID] = true
		}
	}
	expectedLabel := qe.findLabelForVar(toVar, allNodes)

	for _, row := range rows {
		fromNode, ok := row[fromVar]
		if !ok {
			continue
		}

		if inSubgraph != nil && !inSubgraph[fromNode.ID] {
			continue // traversal stays within the subgraph, source included
		}

		visited := map[string]bool{fromNode.ID: true}
		emitted := map[string]bool{}
		frontier := []*types.Node{fromNode}
		for depth := 1; depth <= edgePattern.MaxDepth && len(frontier) > 0; depth++ {
			var next []*types.Node
			for _, cur := range frontier {
				for _, nbID := range qe.neighborIDs(storage, cur.ID, edgePattern, inSubgraph) {
					if visited[nbID] {
						continue
					}
					visited[nbID] = true
					nb, ok := storage.GetNode(nbID)
					if !ok {
						continue
					}
					next = append(next, nb) // traverse through, even past MinDepth or label mismatch
					if depth < minDepth || emitted[nbID] {
						continue
					}
					if expectedLabel != "" && nb.Label != expectedLabel {
						continue
					}
					emitted[nbID] = true
					newRow := copyMap(row)
					newRow[toVar] = nb
					expanded = append(expanded, newRow)
				}
			}
			frontier = next
		}
	}

	return expanded
}

// neighborIDs lists adjacent node IDs honoring direction, kind (empty = any)
// and edge filters; with inSubgraph != nil, nodes outside it are skipped.
func (qe *DefaultQueryEngine) neighborIDs(
	storage storage.StorageEngine,
	nodeID string,
	edgePattern *query.PatternEdge,
	inSubgraph map[string]bool,
) []string {
	var ids []string
	seen := map[string]bool{} // ponytail: per-node dedup; Both union collapses here
	collect := func(edges []*types.Edge, target func(*types.Edge) string) {
		for _, e := range edges {
			if edgePattern.Kind != "" && e.Kind != edgePattern.Kind {
				continue
			}
			if !qe.matchEdgeFilters(e, edgePattern.Filters) {
				continue
			}
			id := target(e)
			if seen[id] {
				continue
			}
			seen[id] = true
			if inSubgraph != nil && !inSubgraph[id] {
				continue
			}
			ids = append(ids, id)
		}
	}
	switch edgePattern.Direction {
	case query.In:
		collect(storage.GetEdgesTo(nodeID), func(e *types.Edge) string { return e.From })
	case query.Both:
		collect(storage.GetEdgesFrom(nodeID), func(e *types.Edge) string { return e.To })
		collect(storage.GetEdgesTo(nodeID), func(e *types.Edge) string { return e.From })
	default: // query.Out
		collect(storage.GetEdgesFrom(nodeID), func(e *types.Edge) string { return e.To })
	}
	return ids
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
