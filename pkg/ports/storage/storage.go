package storage

import "github.com/aprksy/knitknot/pkg/ports/types"

// StorageEngine handles persistence of nodes/edges
type StorageEngine interface {
	AddNode(label string, props map[string]any) (string, error)
	AddEdge(from, to, kind string, props map[string]any) error
	GetNode(id string) (*types.Node, bool)
	GetAllNodes() []*types.Node
	GetAllEdges() []*types.Edge
	GetEdgesFrom(from string) []*types.Edge
	GetEdgesTo(to string) []*types.Edge
	GetEdgesByKind(kind string) []*types.Edge
	GetNodesIn(subgraph string) []*types.Node
	GetEdgesIn(subgraph string) []*types.Edge

	UpdateNode(id string, props map[string]any) error
}
