package query

import (
	"context"

	store "github.com/aprksy/knitknot/pkg/ports/storage"
)

// QueryPlan is internal representation of a query
type QueryPlan struct {
	Nodes     []*PatternNode
	Edges     []*PatternEdge
	Filters   []Filter
	Outputs   []string
	LimitVal  *int
	OffsetVal *int
	Subgraph  string // if non-empty, restrict to this subgraph
}

// QueryEngine compiles and executes queries against a storage engine
type QueryEngine interface {
	Execute(ctx context.Context, storage store.StorageEngine, plan *QueryPlan) (ResultSet, error)
}
