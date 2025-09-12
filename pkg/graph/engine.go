package graph

import (
	"context"

	"github.com/aprksy/knitknot/pkg/ports/query"
	"github.com/aprksy/knitknot/pkg/ports/storage"
	"github.com/aprksy/knitknot/pkg/ports/types"
	q "github.com/aprksy/knitknot/pkg/query"
)

// GraphEngine is the top-level orchestrator that combines storage and query logic.
type GraphEngine struct {
	storage storage.StorageEngine
	query   query.QueryEngine
}

// NewGraphEngine creates a new engine with default components.
func NewGraphEngine(storage storage.StorageEngine) *GraphEngine {
	return &GraphEngine{
		storage: storage,
		query:   q.NewDefaultQueryEngine(),
	}
}

// WithQueryEngine allows replacing the query engine (for testing/plugins).
func (ge *GraphEngine) WithQueryEngine(qe query.QueryEngine) *GraphEngine {
	ge.query = qe
	return ge
}

// AddNode delegates to storage
func (ge *GraphEngine) AddNode(label string, props map[string]any) (string, error) {
	return ge.storage.AddNode(label, props)
}

// AddEdge delegates to storage
func (ge *GraphEngine) AddEdge(from, to, kind string, props map[string]any) error {
	return ge.storage.AddEdge(from, to, kind, props)
}

// GetNode retrieves a node by ID
func (ge *GraphEngine) GetNode(id string) (*types.Node, bool) {
	return ge.storage.GetNode(id)
}

// Query runs a compiled plan using the query engine
func (ge *GraphEngine) Query(ctx context.Context, plan *query.QueryPlan) (query.ResultSet, error) {
	return ge.query.Execute(ctx, ge.storage, plan)
}

// Storage exposes the underlying engine (useful for exporters, debug)
func (ge *GraphEngine) Storage() storage.StorageEngine {
	return ge.storage
}
