// internal/storage/inmem/inmem.go
package inmem

import (
	"errors"
	"fmt"
	"math/rand"
	"slices"
	"sync"

	"github.com/aprksy/knitknot/pkg/ports/storage"
	"github.com/aprksy/knitknot/pkg/ports/types"
)

var _ storage.StorageEngine = (*Storage)(nil)

type Storage struct {
	mu    sync.RWMutex
	nodes map[string]*types.Node
	edges map[string]*types.Edge
}

func New() *Storage {
	return &Storage{
		nodes: make(map[string]*types.Node),
		edges: make(map[string]*types.Edge),
	}
}

func (s *Storage) AddNode(label string, props map[string]any) (string, error) {
	id := generateID()
	node := &types.Node{
		ID:    id,
		Label: label,
		Props: copyMap(props),
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.nodes[id]; exists {
		return "", errors.New("node already exists")
	}
	s.nodes[id] = node
	return id, nil
}

func (s *Storage) AddEdge(from, to, kind string, props map[string]any) error {
	s.mu.RLock()
	_, fromOk := s.nodes[from]
	_, toOk := s.nodes[to]
	s.mu.RUnlock()

	if !fromOk {
		return errors.New("source node not found")
	}
	if !toOk {
		return errors.New("target node not found")
	}

	id := fmt.Sprintf("%s->%s@%s", from, to, kind)
	edge := &types.Edge{
		ID:    id,
		From:  from,
		To:    to,
		Kind:  kind,
		Props: copyMap(props),
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.edges[id] = edge
	return nil
}

func (s *Storage) GetNode(id string) (*types.Node, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n, ok := s.nodes[id]
	return n, ok
}

func (s *Storage) GetAllNodes() []*types.Node {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]*types.Node, 0, len(s.nodes))
	for _, n := range s.nodes {
		list = append(list, n)
	}
	return list
}

func (s *Storage) GetAllEdges() []*types.Edge {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]*types.Edge, 0, len(s.edges))
	for _, e := range s.edges {
		list = append(list, e)
	}
	return list
}

func (s *Storage) GetEdgesFrom(from string) []*types.Edge {
	return s.findEdges(func(e *types.Edge) bool { return e.From == from })
}

func (s *Storage) GetEdgesTo(to string) []*types.Edge {
	return s.findEdges(func(e *types.Edge) bool { return e.To == to })
}

func (s *Storage) GetEdgesByKind(kind string) []*types.Edge {
	return s.findEdges(func(e *types.Edge) bool { return e.Kind == kind })
}

func (s *Storage) findEdges(match func(*types.Edge) bool) []*types.Edge {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*types.Edge
	for _, e := range s.edges {
		if match(e) {
			result = append(result, e)
		}
	}
	return result
}

func (s *Storage) GetNodesIn(subgraph string) []*types.Node {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*types.Node
	for _, n := range s.nodes {
		if slices.Contains(n.Subgraphs, subgraph) {
			result = append(result, n)
		}
	}
	return result
}

func (s *Storage) GetEdgesIn(subgraph string) []*types.Edge {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*types.Edge
	for _, e := range s.edges {
		// Edge belongs to subgraph if both ends do AND edge hasn't been removed
		fromNode, ok1 := s.nodes[e.From]
		toNode, ok2 := s.nodes[e.To]
		if !ok1 || !ok2 {
			continue
		}
		if slices.Contains(fromNode.Subgraphs, subgraph) &&
			slices.Contains(toNode.Subgraphs, subgraph) {
			// Optionally: ensure edge itself includes subgraph
			if slices.Contains(e.Subgraphs, subgraph) {
				result = append(result, e)
			} else {
				// Auto-inherit
				cp := *e
				cp.Subgraphs = append(cp.Subgraphs, subgraph)
				result = append(result, &cp)
			}
		}
	}
	return result
}

// Helpers
func copyMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	cp := make(map[string]any, len(m))
	for k, v := range m {
		cp[k] = v
	}
	return cp
}

func generateID() string {
	// Reuse your ID generator
	return fmt.Sprintf("n%d", rand.Intn(1000000)) // simplify for now
}
