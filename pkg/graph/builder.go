// pkg/graph/builder.go
package graph

import (
	"context"
	"fmt"

	"github.com/aprksy/knitknot/pkg/ports/query"
)

// Builder is the fluent query builder
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

	var targetLabel, propKey string

	// Map relationship type to expected node label + property
	switch rel {
	case "has_skill":
		targetLabel = "Skill"
		propKey = "name"
	case "reports_to":
		targetLabel = "User"
		propKey = "name"
	default:
		targetLabel = "Entity" // fallback
		propKey = "name"
	}

	b.MatchNode(v, targetLabel)
	b.RelatedTo(v, rel, "n")
	b.Where(v+"."+propKey, "=", value)

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

func (b *Builder) freshVar() string {
	id := b.nextVar
	b.nextVar++
	return fmt.Sprintf("v%d", id)
}
