// pkg/graph/builder.go
package graph

import (
	"context"

	"github.com/aprksy/knitknot/pkg/ports/query"
)

// Builder is the fluent query builder (lives in same package → no cycle)
type Builder struct {
	engine  *GraphEngine
	plan    *query.QueryPlan
	nextVar int
}

// Find starts a new query for nodes with given label.
func (ge *GraphEngine) Find(label string) *Builder {
	b := &Builder{
		engine:  ge,
		plan:    &query.QueryPlan{},
		nextVar: 0,
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
	b.MatchNode(v, capitalize(rel))
	b.RelatedTo(v, rel, "n")
	b.Where(v+"."+rel, "=", value)
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

func (b *Builder) Exec(ctx context.Context) (query.ResultSet, error) {
	return b.engine.Query(ctx, b.plan)
}

func (b *Builder) freshVar() string {
	v := b.nextVar
	b.nextVar++
	return "v" + string('a'+rune(v))
}
