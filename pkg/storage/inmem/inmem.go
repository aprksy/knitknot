// internal/storage/inmem/inmem.go
package inmem

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"github.com/aprksy/knitknot/pkg/ports/storage"
	"github.com/aprksy/knitknot/pkg/ports/types"
)

var _ storage.StorageEngine = (*Storage)(nil)
var _ storage.VersionedStorage = (*Storage)(nil) // version: api — inmem implements VersionedStorage

type Storage struct {
	mu           sync.RWMutex
	nodes        map[string]*types.Node
	edges        map[string]*types.Edge
	nodesByLabel map[string]map[string]*types.Node // label -> id -> *Node // perf: index
	// version: edge — single per-Storage log indexing every node+edge mutation.
	eventLog   []types.Event
	revCounter atomic.Uint64
	now        func() time.Time
}

func New() *Storage {
	return &Storage{
		nodes:        make(map[string]*types.Node),
		edges:        make(map[string]*types.Edge),
		nodesByLabel: make(map[string]map[string]*types.Node), // perf: index
		now:          time.Now,
	}
}

func (s *Storage) AddNode(label string, props map[string]any) (string, error) {
	return s.AddNodeWithMeta(label, props, "", "")
}

// version: write-path — AddNodeWithMeta records the initial snapshot + create event.
func (s *Storage) AddNodeWithMeta(label string, props map[string]any, source, transaction string) (string, error) {
	id := generateID()
	now := s.clock()
	rev := int64(s.revCounter.Add(1))
	tx := ensureTx(transaction)
	node := &types.Node{
		ID:         id,
		Label:      label,
		Props:      copyMap(props),
		Subgraphs:  map[string]*types.Subgraph{},
		CreatedRev: rev,
		History: []types.Snapshot{{
			Rev:         rev,
			EventTime:   now,
			LogicalTime: now,
			Props:       copyMap(props),
			Source:      source,
			Transaction: tx,
		}},
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.nodes[id]; exists {
		return "", errors.New("node already exists")
	}
	s.nodes[id] = node
	if s.nodesByLabel[label] == nil { // perf: index
		s.nodesByLabel[label] = make(map[string]*types.Node) // perf: index
	} // perf: index
	s.nodesByLabel[label][id] = node // perf: index
	s.eventLog = append(s.eventLog, types.Event{
		Rev:         rev,
		EventTime:   now,
		LogicalTime: now,
		Op:          "create",
		ElementType: "node",
		ElementID:   id,
		Props:       copyMap(props),
		Source:      source,
		Transaction: tx,
	})
	return id, nil
}

func (s *Storage) AddToSubgraph(n *types.Node, sgName, sgDesc string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	subgraph := &types.Subgraph{
		Name:        sgName,
		Description: sgDesc,
	}
	n.Subgraphs[subgraph.Name] = subgraph
}

func (s *Storage) RemoveFromSubgraph(n *types.Node, sgName string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(n.Subgraphs, sgName)
}

func (s *Storage) AddEdge(from, to, kind string, props map[string]any) error {
	return s.AddEdgeWithMeta(from, to, kind, props, "", "")
}

// version: edge — AddEdgeWithMeta records the initial snapshot + create event.
func (s *Storage) AddEdgeWithMeta(from, to, kind string, props map[string]any, source, transaction string) error {
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

	now := s.clock()
	rev := int64(s.revCounter.Add(1))
	tx := ensureTx(transaction)

	id := fmt.Sprintf("%s->%s@%s", from, to, kind)
	edge := &types.Edge{
		ID:         id,
		From:       from,
		To:         to,
		Kind:       kind,
		Props:      copyMap(props),
		Subgraphs:  map[string]*types.Subgraph{},
		CreatedRev: rev,
		History: []types.Snapshot{{
			Rev:         rev,
			EventTime:   now,
			LogicalTime: now,
			Props:       copyMap(props),
			Source:      source,
			Transaction: tx,
		}},
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.edges[id] = edge
	s.eventLog = append(s.eventLog, types.Event{
		Rev:         rev,
		EventTime:   now,
		LogicalTime: now,
		Op:          "create",
		ElementType: "edge",
		ElementID:   id,
		From:        from,
		To:          to,
		Kind:        kind,
		Props:       copyMap(props),
		Source:      source,
		Transaction: tx,
	})
	return nil
}

func (s *Storage) GetNode(id string) (*types.Node, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n, ok := s.nodes[id]
	return n, ok
}

func (s *Storage) GetEdge(id string) (*types.Edge, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.edges[id]
	return e, ok
}

func (s *Storage) GetAllNodes() []*types.Node {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]*types.Node, 0, len(s.nodes))
	for _, n := range s.nodes {
		if n.DeletedAt != nil { // version: live-only enumeration
			continue
		}
		list = append(list, n)
	}
	return list
}

func (s *Storage) GetNodesByLabel(label string) []*types.Node { // perf: index
	s.mu.RLock()                        // perf: index
	defer s.mu.RUnlock()                // perf: index
	bucket, ok := s.nodesByLabel[label] // perf: index
	if !ok {                            // perf: index
		return []*types.Node{} // perf: index
	} // perf: index
	list := make([]*types.Node, 0, len(bucket)) // perf: index
	for _, n := range bucket {                  // perf: index
		if n.DeletedAt != nil { // version: live-only enumeration
			continue
		}
		list = append(list, n) // perf: index
	} // perf: index
	return list // perf: index
} // perf: index

func (s *Storage) GetAllEdges() []*types.Edge {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]*types.Edge, 0, len(s.edges))
	for _, e := range s.edges {
		if e.DeletedAt != nil { // version: edge — live-only enumeration
			continue
		}
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

func (s *Storage) UpdateNode(id string, props map[string]any) error {
	return s.UpdateNodeWithMeta(id, props, "", "")
}

// version: write-path — UpdateNode appends a snapshot; Props keeps REPLACE semantics.
func (s *Storage) UpdateNodeWithMeta(id string, props map[string]any, source, transaction string) error {
	now := s.clock()
	tx := ensureTx(transaction)

	s.mu.Lock()
	defer s.mu.Unlock()

	node, ok := s.nodes[id]
	if !ok {
		return fmt.Errorf("node not found")
	}
	if node.DeletedAt != nil {
		return fmt.Errorf("node deleted")
	}

	rev := int64(s.revCounter.Add(1))
	node.Props = copyMap(props)
	node.History = append(node.History, types.Snapshot{
		Rev:         rev,
		EventTime:   now,
		LogicalTime: now,
		Props:       copyMap(props),
		Source:      source,
		Transaction: tx,
	})
	s.eventLog = append(s.eventLog, types.Event{
		Rev:         rev,
		EventTime:   now,
		LogicalTime: now,
		Op:          "update",
		ElementType: "node",
		ElementID:   id,
		Props:       copyMap(props),
		Source:      source,
		Transaction: tx,
	})
	return nil
}

func (s *Storage) UpdateEdge(id string, props map[string]any) error {
	return s.UpdateEdgeWithMeta(id, props, "", "")
}

// version: edge — UpdateEdge appends a snapshot; Props keeps REPLACE semantics.
func (s *Storage) UpdateEdgeWithMeta(id string, props map[string]any, source, transaction string) error {
	now := s.clock()
	tx := ensureTx(transaction)

	s.mu.Lock()
	defer s.mu.Unlock()

	edge, ok := s.edges[id]
	if !ok {
		return fmt.Errorf("edge not found")
	}
	if edge.DeletedAt != nil {
		return fmt.Errorf("edge deleted")
	}

	rev := int64(s.revCounter.Add(1))
	edge.Props = copyMap(props)
	edge.History = append(edge.History, types.Snapshot{
		Rev:         rev,
		EventTime:   now,
		LogicalTime: now,
		Props:       copyMap(props),
		Source:      source,
		Transaction: tx,
	})
	s.eventLog = append(s.eventLog, types.Event{
		Rev:         rev,
		EventTime:   now,
		LogicalTime: now,
		Op:          "update",
		ElementType: "edge",
		ElementID:   id,
		From:        edge.From,
		To:          edge.To,
		Kind:        edge.Kind,
		Props:       copyMap(props),
		Source:      source,
		Transaction: tx,
	})
	return nil
}

func (s *Storage) findEdges(match func(*types.Edge) bool) []*types.Edge {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*types.Edge
	for _, e := range s.edges {
		if e.DeletedAt != nil { // version: edge — tombstones hidden centrally
			continue
		}
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
		if n.DeletedAt != nil { // version: live-only enumeration
			continue
		}
		if _, exists := n.Subgraphs[subgraph]; exists {
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
		if e.DeletedAt != nil { // version: edge — live-only enumeration
			continue
		}
		// Edge belongs to subgraph if both ends do AND edge hasn't been removed
		fromNode, ok1 := s.nodes[e.From]
		toNode, ok2 := s.nodes[e.To]
		if !ok1 || !ok2 {
			continue
		}
		sgInstance, fromNodeExists := fromNode.Subgraphs[subgraph]
		_, toNodeExists := toNode.Subgraphs[subgraph]

		if fromNodeExists && toNodeExists {
			// Optionally: ensure edge itself includes subgraph
			if _, exists := e.Subgraphs[subgraph]; exists {
				result = append(result, e)
			} else {
				// Auto-inherit
				cp := *e                                                            // fix: H4
				cp.Subgraphs = make(map[string]*types.Subgraph, len(e.Subgraphs)+1) // fix: H4
				for k, v := range e.Subgraphs {                                     // fix: H4
					cp.Subgraphs[k] = v // fix: H4
				} // fix: H4
				cp.Subgraphs[subgraph] = sgInstance // fix: H4
				result = append(result, &cp)
			}
		}
	}
	return result
}

func (s *Storage) DeleteNode(id string) error {
	return s.DeleteNodeWithMeta(id, "", "")
}

// version: edge — DeleteNode is a tombstone, not a removal.
// The node stays findable via GetNode with DeletedAt set; incident
// edges are tombstoned too (version: cascade-tombstone), never hard-deleted.
func (s *Storage) DeleteNodeWithMeta(id, source, transaction string) error {
	now := s.clock()
	tx := ensureTx(transaction)

	s.mu.Lock()
	defer s.mu.Unlock()
	n, ok := s.nodes[id] // perf: index
	if !ok {             // perf: index
		return fmt.Errorf("node not found")
	}
	if n.DeletedAt != nil {
		return fmt.Errorf("node already deleted")
	}
	rev := int64(s.revCounter.Add(1))
	n.History = append(n.History, types.Snapshot{
		Rev:         rev,
		EventTime:   now,
		LogicalTime: now,
		Props:       copyMap(n.Props),
		Source:      source,
		Transaction: tx,
		Deleted:     true,
	})
	n.DeletedAt = &now
	s.eventLog = append(s.eventLog, types.Event{
		Rev:         rev,
		EventTime:   now,
		LogicalTime: now,
		Op:          "delete",
		ElementType: "node",
		ElementID:   id,
		Props:       copyMap(n.Props),
		Source:      source,
		Transaction: tx,
	})
	// version: cascade-tombstone — incident edges are tombstoned, never hard-deleted.
	// They stay findable via GetEdge with DeletedAt set; live-only
	// enumeration (findEdges/GetAllEdges/GetEdgesIn) already skips them.
	// Edge events share the parent's Source/Transaction so the whole
	// operation groups under one txid.
	for _, e := range s.edges {
		if e.From != id && e.To != id {
			continue
		}
		if e.DeletedAt != nil { // version: cascade-tombstone — skip already-tombstoned edges
			continue
		}
		rev := int64(s.revCounter.Add(1)) // version: cascade-tombstone — one rev per cascaded edge
		e.History = append(e.History, types.Snapshot{
			Rev:         rev,
			EventTime:   now,
			LogicalTime: now,
			Props:       copyMap(e.Props),
			Source:      source,
			Transaction: tx,
			Deleted:     true,
		})
		t := now // version: cascade-tombstone — per-edge DeletedAt copy
		e.DeletedAt = &t
		s.eventLog = append(s.eventLog, types.Event{
			Rev:         rev,
			EventTime:   now,
			LogicalTime: now,
			Op:          "delete",
			ElementType: "edge",
			ElementID:   e.ID,
			From:        e.From,
			To:          e.To,
			Kind:        e.Kind,
			Props:       copyMap(e.Props),
			Source:      source,
			Transaction: tx,
		})
	}
	return nil
}

func (s *Storage) DeleteEdge(from, to, kind string) error {
	return s.DeleteEdgeWithMeta(from, to, kind, "", "")
}

// version: edge — DeleteEdge is a tombstone, not a removal.
// The edge stays findable via GetEdge with DeletedAt set.
func (s *Storage) DeleteEdgeWithMeta(from, to, kind, source, transaction string) error {
	now := s.clock()
	tx := ensureTx(transaction)

	s.mu.Lock()
	defer s.mu.Unlock()
	id := fmt.Sprintf("%s->%s@%s", from, to, kind)
	e, ok := s.edges[id]
	if !ok {
		return fmt.Errorf("edge not found")
	}
	if e.DeletedAt != nil {
		return fmt.Errorf("edge already deleted")
	}
	rev := int64(s.revCounter.Add(1))
	e.History = append(e.History, types.Snapshot{
		Rev:         rev,
		EventTime:   now,
		LogicalTime: now,
		Props:       copyMap(e.Props),
		Source:      source,
		Transaction: tx,
		Deleted:     true,
	})
	e.DeletedAt = &now
	s.eventLog = append(s.eventLog, types.Event{
		Rev:         rev,
		EventTime:   now,
		LogicalTime: now,
		Op:          "delete",
		ElementType: "edge",
		ElementID:   id,
		From:        e.From,
		To:          e.To,
		Kind:        e.Kind,
		Props:       copyMap(e.Props),
		Source:      source,
		Transaction: tx,
	})
	return nil
}

// version: read-path — temporal + event-log queries (ADR 0002).

// GetNodeAt returns the node as of time t: latest snapshot with
// EventTime <= t. Tombstone at t, zero t, or unknown id → not-found.
func (s *Storage) GetNodeAt(id string, t time.Time) (*types.Node, bool) {
	if t.IsZero() {
		return nil, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	n, ok := s.nodes[id]
	if !ok {
		return nil, false
	}
	best := -1
	for i, snap := range n.History {
		if !snap.EventTime.After(t) {
			best = i
		}
	}
	if best < 0 || n.History[best].Deleted {
		return nil, false
	}
	cp := *n
	cp.Props = copyMap(n.History[best].Props)
	cp.DeletedAt = nil
	cp.History = append([]types.Snapshot(nil), n.History[:best+1]...)
	return &cp, true
}

// GetNodeHistory returns the node's full history, newest-last.
func (s *Storage) GetNodeHistory(id string) []types.Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n, ok := s.nodes[id]
	if !ok {
		return nil
	}
	return append([]types.Snapshot(nil), n.History...)
}

// version: edge — temporal reads for edges, mirroring GetNodeAt/GetNodeHistory.

// GetEdgeAt returns the edge as of time t: latest snapshot with
// EventTime <= t. Tombstone at t, zero t, or unknown id → not-found.
func (s *Storage) GetEdgeAt(id string, t time.Time) (*types.Edge, bool) {
	if t.IsZero() {
		return nil, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.edges[id]
	if !ok {
		return nil, false
	}
	best := -1
	for i, snap := range e.History {
		if !snap.EventTime.After(t) {
			best = i
		}
	}
	if best < 0 || e.History[best].Deleted {
		return nil, false
	}
	cp := *e
	cp.Props = copyMap(e.History[best].Props)
	cp.DeletedAt = nil
	cp.History = append([]types.Snapshot(nil), e.History[:best+1]...)
	return &cp, true
}

// GetEdgeHistory returns the edge's full history, newest-last.
func (s *Storage) GetEdgeHistory(id string) []types.Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.edges[id]
	if !ok {
		return nil
	}
	return append([]types.Snapshot(nil), e.History...)
}

// GetEvents returns events in [fromRev, toRev] inclusive, Rev ascending.
// toRev == 0 means "to current rev".
func (s *Storage) GetEvents(fromRev, toRev int64) []types.Event {
	s.mu.RLock()
	defer s.mu.RUnlock()
	hi := toRev
	if hi == 0 {
		hi = int64(s.revCounter.Load())
	}
	var out []types.Event
	for _, e := range s.eventLog {
		if e.Rev >= fromRev && e.Rev <= hi {
			out = append(out, e)
		}
	}
	return out
}

// GetEventsByTransaction returns events sharing a transaction ID, Rev order.
func (s *Storage) GetEventsByTransaction(txID string) []types.Event {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []types.Event
	for _, e := range s.eventLog {
		if e.Transaction == txID {
			out = append(out, e)
		}
	}
	return out
}

// EventsTouching returns events for one element ID, Rev order.
func (s *Storage) EventsTouching(elementID string) []types.Event {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []types.Event
	for _, e := range s.eventLog {
		if e.ElementID == elementID {
			out = append(out, e)
		}
	}
	return out
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

// version: clock — injectable in production via now field; defaults to time.Now.
func (s *Storage) clock() time.Time {
	if s.now != nil {
		return s.now()
	}
	return time.Now()
}

// version: tx — per-call transaction auto-generates a UUID (ADR open decision #3).
func ensureTx(tx string) string {
	if tx != "" {
		return tx
	}
	return uuid.NewString()
}

var idSeq atomic.Uint64

func generateID() string {
	return fmt.Sprintf("n%d", idSeq.Add(1))
}
