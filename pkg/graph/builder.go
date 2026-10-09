// pkg/graph/builder.go
package graph

import (
	"context"
	"fmt"

	"github.com/aprksy/knitknot/pkg/ports/query"
	"github.com/aprksy/knitknot/pkg/ports/types"
)

// Builder is the fluent query builder
type Builder struct {
	engine  *GraphEngine
	plan    *query.QueryPlan
	nextVar int
	lastVar string
}

// Direction re-exports query.Direction so callers need only the graph package.
type Direction = query.Direction

const (
	Out  = query.Out
	In   = query.In
	Both = query.Both
)

// Find starts a new query for nodes with given label.
func (ge *GraphEngine) Find(label string) *Builder {
	b := &Builder{
		engine:  ge,
		plan:    &query.QueryPlan{},
		nextVar: 0,
		lastVar: "n",
	}
	if ge.defaultSubgraph != "" {
		b.plan.Subgraph = ge.defaultSubgraph
	}
	return b.MatchNode("n", label)
}

func (b *Builder) MatchNode(varName, label string) *Builder {
	b.plan.Nodes = append(b.plan.Nodes, &query.PatternNode{
		Var:   varName,
		Label: label,
	})
	if len(b.plan.Outputs) == 0 {
		b.plan.Outputs = append(b.plan.Outputs, varName)
	}
	return b
}

func (b *Builder) Where(field, op string, value any) *Builder {
	b.plan.Filters = append(b.plan.Filters, query.Filter{
		Field: field,
		Op:    op,
		Value: value,
	})
	return b
}

func (b *Builder) Limit(n int) *Builder {
	b.plan.LimitVal = &n
	return b
}

func (b *Builder) Has(rel, value string) *Builder {
	v := b.freshVar()

	// Look up verb semantics
	verb, ok := b.engine.verbs.Lookup(rel)
	if !ok {
		verb = types.Verb{
			TargetLabel: "", // match any label; MatchOn still applies
			MatchOn:     types.DefaultMatchProperty,
		}
	}

	targetLabel := verb.TargetLabel
	// NOTE: empty TargetLabel (multi-target verbs, unknown verbs) means no
	// label constraint — the MatchOn property filter still applies.

	propKey := verb.MatchOn
	if propKey == "" {
		propKey = types.DefaultMatchProperty
	}

	b.MatchNode(v, targetLabel)
	b.RelatedTo(v, rel, "n")
	b.Where(v+"."+propKey, "=", value)
	b.lastVar = v

	return b
}

func (b *Builder) RelatedTo(targetVar, edgeKind, sourceVar string) *Builder {
	b.plan.Edges = append(b.plan.Edges, &query.PatternEdge{
		From: sourceVar,
		To:   targetVar,
		Kind: edgeKind,
	})
	return b
}

// Follow adds one hop along rel from the last matched variable.
func (b *Builder) Follow(rel string, dir query.Direction) *Builder {
	v := b.freshVar()

	verb, ok := b.engine.verbs.Lookup(rel)
	if !ok {
		verb = types.Verb{
			TargetLabel: "", // match any label
			MatchOn:     types.DefaultMatchProperty,
		}
	}

	b.MatchNode(v, verb.TargetLabel)
	b.plan.Edges = append(b.plan.Edges, &query.PatternEdge{
		From:      b.lastVar,
		To:        v,
		Kind:      rel,
		Direction: dir,
	})
	b.lastVar = v

	return b
}

// FollowHas is Follow plus a target name filter (the chaining analog of Has).
func (b *Builder) FollowHas(rel, value string, dir query.Direction) *Builder {
	b.Follow(rel, dir)

	verb, ok := b.engine.verbs.Lookup(rel)
	if !ok {
		verb = types.Verb{MatchOn: types.DefaultMatchProperty}
	}
	propKey := verb.MatchOn
	if propKey == "" {
		propKey = types.DefaultMatchProperty
	}

	b.Where(b.lastVar+"."+propKey, "=", value)

	return b
}

func (b *Builder) WhereEdge(field, op string, value any) *Builder {
	if len(b.plan.Edges) == 0 {
		return b
	}
	edge := b.plan.Edges[len(b.plan.Edges)-1]
	edge.Filters = append(edge.Filters, query.Filter{Field: field, Op: op, Value: value})
	return b
}

func (b *Builder) In(subgraph string) *Builder {
	b.plan.Subgraph = subgraph
	return b
}

func (b *Builder) Exec(ctx context.Context) (query.ResultSet, error) {
	result, err := b.engine.Query(ctx, b.plan)
	return result, err
}

// Only for testing
func (b *Builder) ExportPlanForTest() *query.QueryPlan {
	return b.plan
}

func (b *Builder) freshVar() string {
	id := b.nextVar
	b.nextVar++
	return fmt.Sprintf("v%d", id)
}
