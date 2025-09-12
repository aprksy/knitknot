package query

import (
	"context"

	store "github.com/aprksy/knitknot/pkg/ports/storage"
)

// QueryPlan is internal representation of a query (from earlier)
type QueryPlan struct {
	Nodes     []*PatternNode
	Edges     []*PatternEdge
	Filters   []Filter
	Outputs   []string
	LimitVal  *int
	OffsetVal *int
}

// QueryEngine compiles and executes queries against a storage engine
type QueryEngine interface {
	Execute(ctx context.Context, storage store.StorageEngine, plan *QueryPlan) (ResultSet, error)
}
