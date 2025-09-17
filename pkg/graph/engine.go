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
	storage         storage.StorageEngine
	query           query.QueryEngine
	defaultSubgraph string
	verbs           *types.VerbRegistry
}

// NewGraphEngine creates a new engine with default components.
func NewGraphEngine(storage storage.StorageEngine) *GraphEngine {
	return &GraphEngine{
		storage: storage,
		query:   q.NewDefaultQueryEngine(),
		verbs:   types.NewVerbRegistry(),
	}
}

// WithQueryEngine allows replacing the query engine (for testing/plugins).
func (ge *GraphEngine) WithQueryEngine(qe query.QueryEngine) *GraphEngine {
	ge.query = qe
	return ge
}

func (ge *GraphEngine) WithSubgraph(name string) *GraphEngine {
	ge.defaultSubgraph = name
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

// GetEdge retrieves a edge by ID
func (ge *GraphEngine) GetEdge(id string) (*types.Edge, bool) {
	return ge.storage.GetEdge(id)
}

// Query runs a compiled plan using the query engine
func (ge *GraphEngine) Query(ctx context.Context, plan *query.QueryPlan) (query.ResultSet, error) {
	result, err := ge.query.Execute(ctx, ge.storage, plan)
	return result, err
}

// Storage exposes the underlying engine (useful for exporters, debug)
func (ge *GraphEngine) Storage() storage.StorageEngine {
	return ge.storage
}

// WithVerbs allows replacing or extending the registry
func (ge *GraphEngine) WithVerbs(vr *types.VerbRegistry) *GraphEngine {
	ge.verbs = vr
	return ge
}

// RegisterVerb adds a new relationship semantic
func (ge *GraphEngine) RegisterVerb(name string, def types.Verb) {
	ge.verbs.Register(name, def)
}

// Verbs returns the verb registry (for introspection)
func (ge *GraphEngine) Verbs() *types.VerbRegistry {
	return ge.verbs
}

func (ge *GraphEngine) UpdateNode(id string, props map[string]any) error {
	return ge.storage.UpdateNode(id, props)
}

func (ge *GraphEngine) UpdateEdge(id string, props map[string]any) error {
	return ge.storage.UpdateEdge(id, props)
}

func (ge *GraphEngine) DeleteNode(id string) error {
	return ge.storage.DeleteNode(id)
}

func (ge *GraphEngine) DeleteEdge(from, to, kind string) error {
	return ge.storage.DeleteEdge(from, to, kind)
}
