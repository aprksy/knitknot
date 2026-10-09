// Package bolt is a BoltDB-backed StorageEngine (ADR 0003, P1+P2).
// Drop-in port of pkg/storage/inmem: same method semantics, durable buckets.
package bolt

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	bolt "go.etcd.io/bbolt"

	"github.com/aprksy/knitknot/pkg/ports/storage"
	"github.com/aprksy/knitknot/pkg/ports/types"
)

var _ storage.VersionedStorage = (*Storage)(nil)

func init() {
	gob.Register(map[string]*types.Subgraph{})
	gob.Register([]interface{}{})
	gob.Register(map[string]interface{}{})
}

// Bucket names (ADR 0003). stixIndex/edgeDedup writes land in P3;
// history + events are populated on every write (P2) — history reads
// still serve from the inline node/edge values (single source of truth,
// exact inmem match) while the bucket keeps a durable copy.
var (
	bkNodes        = []byte("nodes")
	bkEdges        = []byte("edges")
	bkNodesByLabel = []byte("nodesByLabel")
	bkEdgesByFrom  = []byte("edgesByFrom")
	bkEdgesByTo    = []byte("edgesByTo")
	bkEdgesByKind  = []byte("edgesByKind")
	bkStixIndex    = []byte("stixIndex")
	bkEdgeDedup    = []byte("edgeDedup")
	bkHistory      = []byte("history")
	bkEvents       = []byte("events")
	bkMeta         = []byte("meta")
)

var allBuckets = [][]byte{
	bkNodes, bkEdges, bkNodesByLabel, bkEdgesByFrom, bkEdgesByTo,
	bkEdgesByKind, bkStixIndex, bkEdgeDedup, bkHistory, bkEvents, bkMeta,
}

var (
	metaRevCounter = []byte("revCounter")
	metaIDSeq      = []byte("idSeq")
)

type Storage struct {
	db      *bolt.DB
	now     func() time.Time
	batchTx *bolt.Tx
}

// Batch runs fn inside a single read-write transaction. Writes issued via
// the Storage methods inside fn reuse the open transaction instead of
// opening one per call; reads serve from it via view.
func (s *Storage) Batch(fn func() error) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		s.batchTx = tx
		defer func() { s.batchTx = nil }()
		return fn()
	})
}

// view runs fn on the open batch transaction when in batch mode,
// otherwise on a fresh read transaction.
func (s *Storage) view(fn func(tx *bolt.Tx) error) error {
	if s.batchTx != nil {
		return fn(s.batchTx)
	}
	return s.db.View(fn)
}

// Open creates/opens the DB at path, creating all buckets.
func Open(path string) (*Storage, error) {
	return open(path, false)
}

// OpenNoSync opens with NoSync mode — skips per-transaction fsync for
// bulk imports. Crash mid-import loses the import (re-run it); the final
// Sync on Close still flushes everything.
func OpenNoSync(path string) (*Storage, error) {
	return open(path, true)
}

func open(path string, noSync bool) (*Storage, error) {
	db, err := bolt.Open(path, 0600, nil)
	if err != nil {
		return nil, err
	}
	if noSync {
		db.NoSync = true
	}
	if err := db.Update(func(tx *bolt.Tx) error {
		for _, name := range allBuckets {
			if _, err := tx.CreateBucketIfNotExists(name); err != nil {
				return fmt.Errorf("create bucket %s: %w", name, err)
			}
		}
		return nil
	}); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Storage{db: db, now: time.Now}, nil
}

func (s *Storage) Close() error { return s.db.Close() }

// OpenTemp opens a process-lifetime database on a temp file for callers
// (like Backend.Engine) that need bolt semantics without a named path.
func OpenTemp() (*Storage, error) {
	f, err := os.CreateTemp("", "knitknot-*.db")
	if err != nil {
		return nil, err
	}
	_ = f.Close()
	return Open(f.Name())
}

func (s *Storage) clock() time.Time {
	if s.now != nil {
		return s.now()
	}
	return time.Now()
}

// --- keys / counters ---

func join(parts ...string) []byte {
	var b []byte
	for i, p := range parts {
		if i > 0 {
			b = append(b, 0)
		}
		b = append(b, p...)
	}
	return b
}

func prefixKey(s string) []byte { return append([]byte(s), 0) }

// historyKey is elemID + \x00 + big-endian rev, so a prefix scan over one
// element yields snapshots in rev order. events are keyed by revKey alone.
func historyKey(elemID string, rev int64) []byte {
	k := append([]byte(elemID), 0)
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(rev))
	return append(k, buf[:]...)
}

func revKey(rev int64) []byte {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(rev))
	return buf[:]
}

func revFromKey(k []byte) int64 { return int64(binary.BigEndian.Uint64(k)) }

func putSnapshot(tx *bolt.Tx, elemID string, snap types.Snapshot) error {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(snap); err != nil {
		return err
	}
	return tx.Bucket(bkHistory).Put(historyKey(elemID, snap.Rev), buf.Bytes())
}

func putEvent(tx *bolt.Tx, ev types.Event) error {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(ev); err != nil {
		return err
	}
	return tx.Bucket(bkEvents).Put(revKey(ev.Rev), buf.Bytes())
}

func decodeEvent(raw []byte) (types.Event, error) {
	var ev types.Event
	err := gob.NewDecoder(bytes.NewReader(raw)).Decode(&ev)
	return ev, err
}

func getCounter(b *bolt.Bucket, key []byte) uint64 {
	v := b.Get(key)
	if len(v) != 8 {
		return 0
	}
	return binary.BigEndian.Uint64(v)
}

func nextCounter(b *bolt.Bucket, key []byte) uint64 {
	n := getCounter(b, key) + 1
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], n)
	if err := b.Put(key, buf[:]); err != nil {
		panic(err) // Bucket.Put fails only on closed tx/DB — programmer error
	}
	return n
}

// --- gob codec (matches inmem persist format) ---

func encodeNode(n *types.Node) ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(n); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decodeNode(raw []byte) (*types.Node, error) {
	var n types.Node
	if err := gob.NewDecoder(bytes.NewReader(raw)).Decode(&n); err != nil {
		return nil, err
	}
	return &n, nil
}

func encodeEdge(e *types.Edge) ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(e); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decodeEdge(raw []byte) (*types.Edge, error) {
	var e types.Edge
	if err := gob.NewDecoder(bytes.NewReader(raw)).Decode(&e); err != nil {
		return nil, err
	}
	return &e, nil
}

func putNode(tx *bolt.Tx, n *types.Node) error {
	raw, err := encodeNode(n)
	if err != nil {
		return err
	}
	return tx.Bucket(bkNodes).Put([]byte(n.ID), raw)
}

func putEdge(tx *bolt.Tx, e *types.Edge) error {
	raw, err := encodeEdge(e)
	if err != nil {
		return err
	}
	return tx.Bucket(bkEdges).Put([]byte(e.ID), raw)
}

func getNode(tx *bolt.Tx, id string) (*types.Node, bool) {
	raw := tx.Bucket(bkNodes).Get([]byte(id))
	if raw == nil {
		return nil, false
	}
	n, err := decodeNode(raw)
	if err != nil {
		return nil, false
	}
	return n, true
}

func getEdge(tx *bolt.Tx, id string) (*types.Edge, bool) {
	raw := tx.Bucket(bkEdges).Get([]byte(id))
	if raw == nil {
		return nil, false
	}
	e, err := decodeEdge(raw)
	if err != nil {
		return nil, false
	}
	return e, true
}

// --- writes ---

func (s *Storage) AddNode(label string, props map[string]any) (string, error) {
	return s.AddNodeWithMeta(label, props, "", "")
}

func (s *Storage) AddNodeWithMeta(label string, props map[string]any, source, transaction string) (string, error) {
	if s.batchTx != nil {
		return s.addNodeWithMetaTx(s.batchTx, label, props, source, transaction)
	}
	var id string
	err := s.db.Update(func(tx *bolt.Tx) error {
		var err error
		id, err = s.addNodeWithMetaTx(tx, label, props, source, transaction)
		return err
	})
	if err != nil {
		return "", err
	}
	return id, nil
}

func (s *Storage) addNodeWithMetaTx(tx *bolt.Tx, label string, props map[string]any, source, transaction string) (string, error) {
	meta := tx.Bucket(bkMeta)
	id := fmt.Sprintf("n%d", nextCounter(meta, metaIDSeq))
	rev := int64(nextCounter(meta, metaRevCounter))
	now := s.clock()
	txID := ensureTx(transaction)
	snap := types.Snapshot{
		Rev:         rev,
		EventTime:   now,
		LogicalTime: now,
		Props:       copyMap(props),
		Source:      source,
		Transaction: txID,
	}
	node := &types.Node{
		ID:         id,
		Label:      label,
		Props:      copyMap(props),
		Subgraphs:  map[string]*types.Subgraph{},
		CreatedRev: rev,
		History:    []types.Snapshot{snap},
	}
	if err := putNode(tx, node); err != nil {
		return "", err
	}
	if err := tx.Bucket(bkNodesByLabel).Put(join(label, id), nil); err != nil {
		return "", err
	}
	if err := putSnapshot(tx, id, snap); err != nil {
		return "", err
	}
	err := putEvent(tx, types.Event{
		Rev:         rev,
		EventTime:   now,
		LogicalTime: now,
		Op:          "create",
		ElementType: "node",
		ElementID:   id,
		Source:      source,
		Transaction: txID,
	})
	if err != nil {
		return "", err
	}
	return id, nil
}

func (s *Storage) AddEdge(from, to, kind string, props map[string]any) error {
	return s.AddEdgeWithMeta(from, to, kind, props, "", "")
}

func (s *Storage) AddEdgeWithMeta(from, to, kind string, props map[string]any, source, transaction string) error {
	if s.batchTx != nil {
		return s.addEdgeWithMetaTx(s.batchTx, from, to, kind, props, source, transaction)
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		return s.addEdgeWithMetaTx(tx, from, to, kind, props, source, transaction)
	})
}

func (s *Storage) addEdgeWithMetaTx(tx *bolt.Tx, from, to, kind string, props map[string]any, source, transaction string) error {
	if _, ok := getNode(tx, from); !ok {
		return errors.New("source node not found")
	}
	if _, ok := getNode(tx, to); !ok {
		return errors.New("target node not found")
	}
	meta := tx.Bucket(bkMeta)
	rev := int64(nextCounter(meta, metaRevCounter))
	now := s.clock()
	txID := ensureTx(transaction)
	id := fmt.Sprintf("%s->%s@%s", from, to, kind)
	snap := types.Snapshot{
		Rev:         rev,
		EventTime:   now,
		LogicalTime: now,
		Props:       copyMap(props),
		Source:      source,
		Transaction: txID,
	}
	edge := &types.Edge{
		ID:         id,
		From:       from,
		To:         to,
		Kind:       kind,
		Props:      copyMap(props),
		Subgraphs:  map[string]*types.Subgraph{},
		CreatedRev: rev,
		History:    []types.Snapshot{snap},
	}
	if err := putEdge(tx, edge); err != nil {
		return err
	}
	if err := tx.Bucket(bkEdgesByFrom).Put(join(from, id), nil); err != nil {
		return err
	}
	if err := tx.Bucket(bkEdgesByTo).Put(join(to, id), nil); err != nil {
		return err
	}
	if err := tx.Bucket(bkEdgesByKind).Put(join(kind, id), nil); err != nil {
		return err
	}
	if err := tx.Bucket(bkEdgeDedup).Put(join(from, to, kind), []byte(id)); err != nil {
		return err
	}
	if err := putSnapshot(tx, id, snap); err != nil {
		return err
	}
	return putEvent(tx, types.Event{
		Rev:         rev,
		EventTime:   now,
		LogicalTime: now,
		Op:          "create",
		ElementType: "edge",
		ElementID:   id,
		From:        from,
		To:          to,
		Kind:        kind,
		Source:      source,
		Transaction: txID,
	})
}

func (s *Storage) UpdateNode(id string, props map[string]any) error {
	return s.UpdateNodeWithMeta(id, props, "", "")
}

func (s *Storage) UpdateNodeWithMeta(id string, props map[string]any, source, transaction string) error {
	if s.batchTx != nil {
		return s.updateNodeWithMetaTx(s.batchTx, id, props, source, transaction)
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		return s.updateNodeWithMetaTx(tx, id, props, source, transaction)
	})
}

func (s *Storage) updateNodeWithMetaTx(tx *bolt.Tx, id string, props map[string]any, source, transaction string) error {
	node, ok := getNode(tx, id)
	if !ok {
		return fmt.Errorf("node not found")
	}
	if node.DeletedAt != nil {
		return fmt.Errorf("node deleted")
	}
	rev := int64(nextCounter(tx.Bucket(bkMeta), metaRevCounter))
	now := s.clock()
	txID := ensureTx(transaction)
	snap := types.Snapshot{
		Rev:         rev,
		EventTime:   now,
		LogicalTime: now,
		Props:       copyMap(props),
		Source:      source,
		Transaction: txID,
	}
	node.Props = copyMap(props)
	node.History = append(node.History, snap)
	if err := putNode(tx, node); err != nil {
		return err
	}
	if err := putSnapshot(tx, id, snap); err != nil {
		return err
	}
	return putEvent(tx, types.Event{
		Rev:         rev,
		EventTime:   now,
		LogicalTime: now,
		Op:          "update",
		ElementType: "node",
		ElementID:   id,
		Source:      source,
		Transaction: txID,
	})
}

func (s *Storage) UpdateEdge(id string, props map[string]any) error {
	return s.UpdateEdgeWithMeta(id, props, "", "")
}

func (s *Storage) UpdateEdgeWithMeta(id string, props map[string]any, source, transaction string) error {
	if s.batchTx != nil {
		return s.updateEdgeWithMetaTx(s.batchTx, id, props, source, transaction)
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		return s.updateEdgeWithMetaTx(tx, id, props, source, transaction)
	})
}

func (s *Storage) updateEdgeWithMetaTx(tx *bolt.Tx, id string, props map[string]any, source, transaction string) error {
	edge, ok := getEdge(tx, id)
	if !ok {
		return fmt.Errorf("edge not found")
	}
	if edge.DeletedAt != nil {
		return fmt.Errorf("edge deleted")
	}
	rev := int64(nextCounter(tx.Bucket(bkMeta), metaRevCounter))
	now := s.clock()
	txID := ensureTx(transaction)
	snap := types.Snapshot{
		Rev:         rev,
		EventTime:   now,
		LogicalTime: now,
		Props:       copyMap(props),
		Source:      source,
		Transaction: txID,
	}
	edge.Props = copyMap(props)
	edge.History = append(edge.History, snap)
	if err := putEdge(tx, edge); err != nil {
		return err
	}
	if err := putSnapshot(tx, id, snap); err != nil {
		return err
	}
	return putEvent(tx, types.Event{
		Rev:         rev,
		EventTime:   now,
		LogicalTime: now,
		Op:          "update",
		ElementType: "edge",
		ElementID:   id,
		From:        edge.From,
		To:          edge.To,
		Kind:        edge.Kind,
		Source:      source,
		Transaction: txID,
	})
}

func (s *Storage) DeleteNode(id string) error {
	return s.DeleteNodeWithMeta(id, "", "")
}

func (s *Storage) DeleteNodeWithMeta(id, source, transaction string) error {
	if s.batchTx != nil {
		return s.deleteNodeWithMetaTx(s.batchTx, id, source, transaction)
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		return s.deleteNodeWithMetaTx(tx, id, source, transaction)
	})
}

func (s *Storage) deleteNodeWithMetaTx(tx *bolt.Tx, id, source, transaction string) error {
	n, ok := getNode(tx, id)
	if !ok {
		return fmt.Errorf("node not found")
	}
	if n.DeletedAt != nil {
		return fmt.Errorf("node already deleted")
	}
	meta := tx.Bucket(bkMeta)
	now := s.clock()
	txID := ensureTx(transaction)
	rev := int64(nextCounter(meta, metaRevCounter))
	n.History = append(n.History, types.Snapshot{
		Rev:         rev,
		EventTime:   now,
		LogicalTime: now,
		Props:       copyMap(n.Props),
		Source:      source,
		Transaction: txID,
		Deleted:     true,
	})
	n.DeletedAt = &now
	if err := putNode(tx, n); err != nil {
		return err
	}
	if err := putEvent(tx, types.Event{
		Rev:         rev,
		EventTime:   now,
		LogicalTime: now,
		Op:          "delete",
		ElementType: "node",
		ElementID:   id,
		Source:      source,
		Transaction: txID,
	}); err != nil {
		return err
	}
	// version: cascade-tombstone — mirror inmem: tombstone incident
	// edges (dedupe self-loops present in both from/to indexes).
	seen := map[string]struct{}{}
	for _, b := range []*bolt.Bucket{tx.Bucket(bkEdgesByFrom), tx.Bucket(bkEdgesByTo)} {
		p := prefixKey(id)
		c := b.Cursor()
		for k, _ := c.Seek(p); k != nil && bytes.HasPrefix(k, p); k, _ = c.Next() {
			edgeID := string(k[bytes.LastIndexByte(k, 0)+1:])
			if _, dup := seen[edgeID]; dup {
				continue
			}
			seen[edgeID] = struct{}{}
			e, ok := getEdge(tx, edgeID)
			if !ok || e.DeletedAt != nil {
				continue
			}
			erev := int64(nextCounter(meta, metaRevCounter))
			e.History = append(e.History, types.Snapshot{
				Rev:         erev,
				EventTime:   now,
				LogicalTime: now,
				Props:       copyMap(e.Props),
				Source:      source,
				Transaction: txID,
				Deleted:     true,
			})
			t := now
			e.DeletedAt = &t
			if err := putEdge(tx, e); err != nil {
				return err
			}
			if err := putEvent(tx, types.Event{
				Rev:         erev,
				EventTime:   now,
				LogicalTime: now,
				Op:          "delete",
				ElementType: "edge",
				ElementID:   e.ID,
				From:        e.From,
				To:          e.To,
				Kind:        e.Kind,
				Source:      source,
				Transaction: txID,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Storage) DeleteEdge(from, to, kind string) error {
	return s.DeleteEdgeWithMeta(from, to, kind, "", "")
}

func (s *Storage) DeleteEdgeWithMeta(from, to, kind, source, transaction string) error {
	id := fmt.Sprintf("%s->%s@%s", from, to, kind)
	if s.batchTx != nil {
		return s.deleteEdgeWithMetaTx(s.batchTx, id, source, transaction)
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		return s.deleteEdgeWithMetaTx(tx, id, source, transaction)
	})
}

func (s *Storage) deleteEdgeWithMetaTx(tx *bolt.Tx, id, source, transaction string) error {
	e, ok := getEdge(tx, id)
	if !ok {
		return fmt.Errorf("edge not found")
	}
	if e.DeletedAt != nil {
		return fmt.Errorf("edge already deleted")
	}
	rev := int64(nextCounter(tx.Bucket(bkMeta), metaRevCounter))
	now := s.clock()
	txID := ensureTx(transaction)
	e.History = append(e.History, types.Snapshot{
		Rev:         rev,
		EventTime:   now,
		LogicalTime: now,
		Props:       copyMap(e.Props),
		Source:      source,
		Transaction: txID,
		Deleted:     true,
	})
	e.DeletedAt = &now
	if err := putEdge(tx, e); err != nil {
		return err
	}
	return putEvent(tx, types.Event{
		Rev:         rev,
		EventTime:   now,
		LogicalTime: now,
		Op:          "delete",
		ElementType: "edge",
		ElementID:   id,
		From:        e.From,
		To:          e.To,
		Kind:        e.Kind,
		Source:      source,
		Transaction: txID,
	})
}

// AddToSubgraph / RemoveFromSubgraph mirror the inmem helpers (load-modify-store).
func (s *Storage) AddToSubgraph(nodeID, sgName, sgDesc string) error {
	if s.batchTx != nil {
		return s.addToSubgraphTx(s.batchTx, nodeID, sgName, sgDesc)
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		return s.addToSubgraphTx(tx, nodeID, sgName, sgDesc)
	})
}

func (s *Storage) addToSubgraphTx(tx *bolt.Tx, nodeID, sgName, sgDesc string) error {
	n, ok := getNode(tx, nodeID)
	if !ok {
		return fmt.Errorf("node not found")
	}
	if n.Subgraphs == nil {
		n.Subgraphs = map[string]*types.Subgraph{}
	}
	n.Subgraphs[sgName] = &types.Subgraph{Name: sgName, Description: sgDesc}
	return putNode(tx, n)
}

func (s *Storage) RemoveFromSubgraph(nodeID, sgName string) error {
	if s.batchTx != nil {
		return s.removeFromSubgraphTx(s.batchTx, nodeID, sgName)
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		return s.removeFromSubgraphTx(tx, nodeID, sgName)
	})
}

func (s *Storage) removeFromSubgraphTx(tx *bolt.Tx, nodeID, sgName string) error {
	n, ok := getNode(tx, nodeID)
	if !ok {
		return fmt.Errorf("node not found")
	}
	delete(n.Subgraphs, sgName)
	return putNode(tx, n)
}

// --- reads ---

func (s *Storage) GetNode(id string) (*types.Node, bool) {
	var n *types.Node
	_ = s.view(func(tx *bolt.Tx) error {
		var ok bool
		n, ok = getNode(tx, id)
		if !ok {
			n = nil
		}
		return nil
	})
	return n, n != nil
}

func (s *Storage) GetEdge(id string) (*types.Edge, bool) {
	var e *types.Edge
	_ = s.view(func(tx *bolt.Tx) error {
		var ok bool
		e, ok = getEdge(tx, id)
		if !ok {
			e = nil
		}
		return nil
	})
	return e, e != nil
}

func (s *Storage) GetAllNodes() []*types.Node {
	var out []*types.Node
	_ = s.view(func(tx *bolt.Tx) error {
		c := tx.Bucket(bkNodes).Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			n, err := decodeNode(v)
			if err != nil || n.DeletedAt != nil {
				continue
			}
			out = append(out, n)
		}
		return nil
	})
	return out
}

func (s *Storage) GetAllEdges() []*types.Edge {
	var out []*types.Edge
	_ = s.view(func(tx *bolt.Tx) error {
		c := tx.Bucket(bkEdges).Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			e, err := decodeEdge(v)
			if err != nil || e.DeletedAt != nil {
				continue
			}
			out = append(out, e)
		}
		return nil
	})
	return out
}

// scanIndex resolves each composite key in bucket with the given prefix to
// its edge/node value, skipping tombstones (live-only, like inmem).
func scanEdges(tx *bolt.Tx, bucket []byte, prefix string) []*types.Edge {
	out := []*types.Edge{}
	p := prefixKey(prefix)
	c := tx.Bucket(bucket).Cursor()
	for k, _ := c.Seek(p); k != nil && bytes.HasPrefix(k, p); k, _ = c.Next() {
		e, ok := getEdge(tx, string(k[len(p):]))
		if !ok || e.DeletedAt != nil {
			continue
		}
		out = append(out, e)
	}
	return out
}

func (s *Storage) GetNodesByLabel(label string) []*types.Node {
	out := []*types.Node{}
	_ = s.view(func(tx *bolt.Tx) error {
		p := prefixKey(label)
		c := tx.Bucket(bkNodesByLabel).Cursor()
		for k, _ := c.Seek(p); k != nil && bytes.HasPrefix(k, p); k, _ = c.Next() {
			n, ok := getNode(tx, string(k[len(p):]))
			if !ok || n.DeletedAt != nil {
				continue
			}
			out = append(out, n)
		}
		return nil
	})
	return out
}

func (s *Storage) GetEdgesFrom(from string) []*types.Edge {
	var out []*types.Edge
	_ = s.view(func(tx *bolt.Tx) error {
		out = scanEdges(tx, bkEdgesByFrom, from)
		return nil
	})
	return out
}

func (s *Storage) GetEdgesTo(to string) []*types.Edge {
	var out []*types.Edge
	_ = s.view(func(tx *bolt.Tx) error {
		out = scanEdges(tx, bkEdgesByTo, to)
		return nil
	})
	return out
}

func (s *Storage) GetEdgesByKind(kind string) []*types.Edge {
	var out []*types.Edge
	_ = s.view(func(tx *bolt.Tx) error {
		out = scanEdges(tx, bkEdgesByKind, kind)
		return nil
	})
	return out
}

func (s *Storage) GetNodesIn(subgraph string) []*types.Node {
	var out []*types.Node
	_ = s.view(func(tx *bolt.Tx) error {
		c := tx.Bucket(bkNodes).Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			n, err := decodeNode(v)
			if err != nil || n.DeletedAt != nil {
				continue
			}
			if _, ok := n.Subgraphs[subgraph]; ok {
				out = append(out, n)
			}
		}
		return nil
	})
	return out
}

func (s *Storage) GetEdgesIn(subgraph string) []*types.Edge {
	var out []*types.Edge
	_ = s.view(func(tx *bolt.Tx) error {
		b := tx.Bucket(bkEdges)
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			e, err := decodeEdge(v)
			if err != nil || e.DeletedAt != nil {
				continue
			}
			fromNode, ok1 := getNode(tx, e.From)
			toNode, ok2 := getNode(tx, e.To)
			if !ok1 || !ok2 {
				continue
			}
			sg, fromIn := fromNode.Subgraphs[subgraph]
			_, toIn := toNode.Subgraphs[subgraph]
			if !(fromIn && toIn) {
				continue
			}
			if _, ok := e.Subgraphs[subgraph]; ok {
				out = append(out, e)
			} else {
				cp := *e // auto-inherit, mirroring inmem
				cp.Subgraphs = make(map[string]*types.Subgraph, len(e.Subgraphs)+1)
				for sk, sv := range e.Subgraphs {
					cp.Subgraphs[sk] = sv
				}
				cp.Subgraphs[subgraph] = sg
				out = append(out, &cp)
			}
		}
		return nil
	})
	return out
}

// --- versioned reads (P2): history served from the inline values for an
// exact inmem match; events served from the events bucket. ---

func (s *Storage) GetNodeAt(id string, t time.Time) (*types.Node, bool) {
	if t.IsZero() {
		return nil, false
	}
	n, ok := s.GetNode(id)
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

func (s *Storage) GetNodeHistory(id string) []types.Snapshot {
	n, ok := s.GetNode(id)
	if !ok {
		return nil
	}
	return append([]types.Snapshot(nil), n.History...)
}

func (s *Storage) GetEdgeAt(id string, t time.Time) (*types.Edge, bool) {
	if t.IsZero() {
		return nil, false
	}
	e, ok := s.GetEdge(id)
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

func (s *Storage) GetEdgeHistory(id string) []types.Snapshot {
	e, ok := s.GetEdge(id)
	if !ok {
		return nil
	}
	return append([]types.Snapshot(nil), e.History...)
}

func (s *Storage) GetEvents(fromRev, toRev int64) []types.Event {
	var out []types.Event
	_ = s.view(func(tx *bolt.Tx) error {
		hi := toRev
		if hi == 0 {
			hi = int64(getCounter(tx.Bucket(bkMeta), metaRevCounter))
		}
		c := tx.Bucket(bkEvents).Cursor()
		for k, v := c.Seek(revKey(fromRev)); k != nil && revFromKey(k) <= hi; k, v = c.Next() {
			ev, err := decodeEvent(v)
			if err != nil {
				continue
			}
			out = append(out, ev)
		}
		return nil
	})
	return out
}

func scanEvents(s *Storage, match func(*types.Event) bool) []types.Event {
	var out []types.Event
	_ = s.view(func(tx *bolt.Tx) error {
		c := tx.Bucket(bkEvents).Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			ev, err := decodeEvent(v)
			if err != nil {
				continue
			}
			if match(&ev) {
				out = append(out, ev)
			}
		}
		return nil
	})
	return out
}

func (s *Storage) GetEventsByTransaction(txID string) []types.Event {
	return scanEvents(s, func(ev *types.Event) bool { return ev.Transaction == txID })
}

func (s *Storage) EventsTouching(elementID string) []types.Event {
	return scanEvents(s, func(ev *types.Event) bool { return ev.ElementID == elementID })
}

// ensureTx mirrors inmem: an empty transaction auto-generates a UUID.
func ensureTx(tx string) string {
	if tx != "" {
		return tx
	}
	return uuid.NewString()
}

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
