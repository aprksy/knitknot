package bolt

import (
	"path/filepath"
	"testing"
	"time"

	bolt "go.etcd.io/bbolt"
)

func openTest(t *testing.T) *Storage {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestBoltCRUD(t *testing.T) {
	s := openTest(t)

	// create
	a, err := s.AddNode("person", map[string]any{"name": "a"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.AddNode("person", map[string]any{"name": "b"})
	if err != nil {
		t.Fatal(err)
	}
	c, err := s.AddNode("org", map[string]any{"name": "c"})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.AddEdge(a, b, "knows", map[string]any{"w": 1}); err != nil {
		t.Fatal(err)
	}
	edgeID := a + "->" + b + "@knows"
	if err := s.AddEdge(b, c, "works-for", nil); err != nil {
		t.Fatal(err)
	}

	// read
	if n, ok := s.GetNode(a); !ok || n.Label != "person" || n.Props["name"] != "a" {
		t.Fatalf("GetNode: %+v %v", n, ok)
	}
	if _, ok := s.GetNode("missing"); ok {
		t.Fatal("GetNode missing should miss")
	}
	if e, ok := s.GetEdge(edgeID); !ok || e.Kind != "knows" {
		t.Fatalf("GetEdge: %+v %v", e, ok)
	}
	if got := s.GetNodesByLabel("person"); len(got) != 2 {
		t.Fatalf("GetNodesByLabel: %d", len(got))
	}
	if got := s.GetAllNodes(); len(got) != 3 {
		t.Fatalf("GetAllNodes: %d", len(got))
	}
	if got := s.GetEdgesFrom(a); len(got) != 1 {
		t.Fatalf("GetEdgesFrom: %d", len(got))
	}
	if got := s.GetEdgesTo(c); len(got) != 1 {
		t.Fatalf("GetEdgesTo: %d", len(got))
	}
	if got := s.GetEdgesByKind("knows"); len(got) != 1 {
		t.Fatalf("GetEdgesByKind: %d", len(got))
	}
	if got := s.GetAllEdges(); len(got) != 2 {
		t.Fatalf("GetAllEdges: %d", len(got))
	}

	// update (REPLACE semantics + history grows)
	if err := s.UpdateNode(a, map[string]any{"name": "a2"}); err != nil {
		t.Fatal(err)
	}
	if n, _ := s.GetNode(a); n.Props["name"] != "a2" || len(n.History) != 2 {
		t.Fatalf("UpdateNode: %+v", n)
	}
	if err := s.UpdateEdge(edgeID, map[string]any{"w": 2}); err != nil {
		t.Fatal(err)
	}
	if e, _ := s.GetEdge(edgeID); e.Props["w"] != 2 || len(e.History) != 2 {
		t.Fatalf("UpdateEdge: %+v", e)
	}
	if err := s.UpdateNode("missing", nil); err == nil {
		t.Fatal("UpdateNode missing should fail")
	}

	// subgraphs
	for _, id := range []string{a, b} {
		if err := s.AddToSubgraph(id, "sg1", "d"); err != nil {
			t.Fatal(err)
		}
	}
	if got := s.GetNodesIn("sg1"); len(got) != 2 {
		t.Fatalf("GetNodesIn: %d", len(got))
	}
	if got := s.GetEdgesIn("sg1"); len(got) != 1 { // auto-inherit
		t.Fatalf("GetEdgesIn: %d", len(got))
	}

	// delete edge: tombstone stays findable, hidden from live queries
	if err := s.DeleteEdge(a, b, "knows"); err != nil {
		t.Fatal(err)
	}
	if e, ok := s.GetEdge(edgeID); !ok || e.DeletedAt == nil {
		t.Fatal("deleted edge should stay findable with DeletedAt set")
	}
	if got := s.GetAllEdges(); len(got) != 1 {
		t.Fatalf("GetAllEdges after delete: %d", len(got))
	}
	if got := s.GetEdgesFrom(a); len(got) != 0 {
		t.Fatalf("GetEdgesFrom after delete: %d", len(got))
	}

	// delete node: tombstone + cascade to incident edge b->c
	if err := s.DeleteNode(b); err != nil {
		t.Fatal(err)
	}
	if n, ok := s.GetNode(b); !ok || n.DeletedAt == nil {
		t.Fatal("deleted node should stay findable with DeletedAt set")
	}
	if got := s.GetAllNodes(); len(got) != 2 {
		t.Fatalf("GetAllNodes after delete: %d", len(got))
	}
	if got := s.GetNodesByLabel("person"); len(got) != 1 {
		t.Fatalf("GetNodesByLabel after delete: %d", len(got))
	}
	if e, ok := s.GetEdge(b + "->" + c + "@works-for"); !ok || e.DeletedAt == nil {
		t.Fatal("incident edge should be cascade-tombstoned")
	}
	if got := s.GetAllEdges(); len(got) != 0 {
		t.Fatalf("GetAllEdges after cascade: %d", len(got))
	}
	if err := s.DeleteNode(b); err == nil {
		t.Fatal("double delete should fail")
	}
}

func TestBoltPersistsAcrossReopen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "re.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	a, err := s.AddNode("person", map[string]any{"name": "a"})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	n, ok := s2.GetNode(a)
	if !ok || n.Props["name"] != "a" {
		t.Fatalf("reopen GetNode: %+v %v", n, ok)
	}
	// idSeq survived: next id must not collide
	a2, err := s2.AddNode("person", nil)
	if err != nil {
		t.Fatal(err)
	}
	if a2 == a {
		t.Fatal("idSeq not persisted: duplicate id")
	}
}

func TestBoltVersioned(t *testing.T) {
	s := openTest(t)

	// controllable clock: one tick per write, recorded for time-travel asserts
	base := time.Now()
	tick := 0
	var times []time.Time
	s.now = func() time.Time {
		tick++
		tm := base.Add(time.Duration(tick) * time.Second)
		times = append(times, tm)
		return tm
	}

	a, err := s.AddNode("person", map[string]any{"v": 1}) // tick 1, rev 1
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.AddNode("person", map[string]any{"v": 1}) // tick 2, rev 2
	if err != nil {
		t.Fatal(err)
	}
	edgeID := a + "->" + b + "@knows"
	if err := s.AddEdge(a, b, "knows", map[string]any{"w": 1}); err != nil { // tick 3, rev 3
		t.Fatal(err)
	}
	if err := s.UpdateNode(a, map[string]any{"v": 2}); err != nil { // tick 4, rev 4
		t.Fatal(err)
	}
	if err := s.UpdateEdge(edgeID, map[string]any{"w": 2}); err != nil { // tick 5, rev 5
		t.Fatal(err)
	}
	t1, t4, t5 := times[0], times[3], times[4]

	// GetNodeHistory: full, newest-last
	h := s.GetNodeHistory(a)
	if len(h) != 2 || h[0].Rev != 1 || h[1].Rev != 4 {
		t.Fatalf("GetNodeHistory: %+v", h)
	}
	if s.GetNodeHistory("missing") != nil {
		t.Fatal("GetNodeHistory missing should be nil")
	}
	// GetEdgeHistory
	eh := s.GetEdgeHistory(edgeID)
	if len(eh) != 2 || eh[1].Props["w"] != 2 {
		t.Fatalf("GetEdgeHistory: %+v", eh)
	}

	// GetNodeAt: latest snapshot <= t
	if n, ok := s.GetNodeAt(a, t1); !ok || n.Props["v"] != 1 || len(n.History) != 1 {
		t.Fatalf("GetNodeAt t1: %+v %v", n, ok)
	}
	if n, ok := s.GetNodeAt(a, t4); !ok || n.Props["v"] != 2 || len(n.History) != 2 {
		t.Fatalf("GetNodeAt t4: %+v %v", n, ok)
	}
	if _, ok := s.GetNodeAt(a, base); ok {
		t.Fatal("GetNodeAt before create should miss")
	}
	if _, ok := s.GetNodeAt(a, time.Time{}); ok {
		t.Fatal("GetNodeAt zero time should miss")
	}
	if _, ok := s.GetNodeAt("missing", t4); ok {
		t.Fatal("GetNodeAt missing should miss")
	}
	// GetEdgeAt
	if e, ok := s.GetEdgeAt(edgeID, t5); !ok || e.Props["w"] != 2 {
		t.Fatalf("GetEdgeAt: %+v %v", e, ok)
	}
	if _, ok := s.GetEdgeAt(edgeID, t1); ok {
		t.Fatal("GetEdgeAt before edge create should miss")
	}

	// GetEvents: [fromRev, toRev], ascending; 0 = to current
	all := s.GetEvents(1, 0)
	if len(all) != 5 {
		t.Fatalf("GetEvents(1,0): %d", len(all))
	}
	for i := 1; i < len(all); i++ {
		if all[i].Rev <= all[i-1].Rev {
			t.Fatalf("events not ascending: %+v", all)
		}
	}
	if all[0].Op != "create" || all[0].ElementType != "node" || all[0].ElementID != a {
		t.Fatalf("first event: %+v", all[0])
	}
	if all[2].Op != "create" || all[2].ElementType != "edge" || all[2].From != a {
		t.Fatalf("edge create event: %+v", all[2])
	}
	sub := s.GetEvents(2, 3)
	if len(sub) != 2 || sub[0].Rev != 2 || sub[1].Rev != 3 {
		t.Fatalf("GetEvents(2,3): %+v", sub)
	}
	if got := s.GetEvents(99, 0); len(got) != 0 {
		t.Fatalf("GetEvents beyond head: %+v", got)
	}

	// GetEventsByTransaction: create tx groups its single event
	txID := h[0].Transaction
	if txID == "" {
		t.Fatal("snapshot missing transaction")
	}
	if got := s.GetEventsByTransaction(txID); len(got) != 1 || got[0].ElementID != a {
		t.Fatalf("GetEventsByTransaction: %+v", got)
	}
	// EventsTouching
	if got := s.EventsTouching(a); len(got) != 2 { // node create + node update
		t.Fatalf("EventsTouching node: %+v", got)
	}
	if got := s.EventsTouching(edgeID); len(got) != 2 { // edge create + edge update
		t.Fatalf("EventsTouching edge: %+v", got)
	}

	// history + events buckets populated on disk
	_ = s.db.View(func(tx *bolt.Tx) error {
		nh, ne := 0, 0
		c := tx.Bucket(bkHistory).Cursor()
		for k, _ := c.First(); k != nil; k, _ = c.Next() {
			nh++
		}
		ec := tx.Bucket(bkEvents).Cursor()
		for k, _ := ec.First(); k != nil; k, _ = ec.Next() {
			ne++
		}
		if nh != 5 || ne != 5 { // 2 node + 1 edge creates, 1 node + 1 edge update
			t.Fatalf("buckets: history=%d events=%d", nh, ne)
		}
		return nil
	})

	// delete: tombstone hides At-reads, history keeps the tombstone snapshot
	if err := s.DeleteEdge(a, b, "knows"); err != nil { // tick 6, rev 6
		t.Fatal(err)
	}
	if _, ok := s.GetEdgeAt(edgeID, times[5]); ok {
		t.Fatal("GetEdgeAt at tombstone should miss")
	}
	if e, ok := s.GetEdgeAt(edgeID, t5); !ok || e.Props["w"] != 2 {
		t.Fatalf("GetEdgeAt before delete: %+v %v", e, ok)
	}
	if eh := s.GetEdgeHistory(edgeID); len(eh) != 3 || !eh[2].Deleted {
		t.Fatalf("GetEdgeHistory after delete: %+v", eh)
	}
	if err := s.DeleteNode(a); err != nil { // tick 7, rev 7 node + rev 8 cascade (edge already dead: no cascade)
		t.Fatal(err)
	}
	if _, ok := s.GetNodeAt(a, times[6]); ok {
		t.Fatal("GetNodeAt at tombstone should miss")
	}
	if n, ok := s.GetNodeAt(a, t4); !ok || n.Props["v"] != 2 {
		t.Fatalf("GetNodeAt before delete: %+v %v", n, ok)
	}
	if h := s.GetNodeHistory(a); len(h) != 3 || !h[2].Deleted {
		t.Fatalf("GetNodeHistory after delete: %+v", h)
	}
	if all := s.GetEvents(1, 0); len(all) != 7 {
		t.Fatalf("GetEvents after deletes: %d", len(all))
	}
	if got := s.EventsTouching(a); len(got) != 3 {
		t.Fatalf("EventsTouching after delete: %+v", got)
	}
}

func TestBoltDeleteNodeCascadeEvents(t *testing.T) {
	s := openTest(t)
	a, _ := s.AddNode("n", nil)                       // rev 1
	b, _ := s.AddNode("n", nil)                       // rev 2
	if err := s.AddEdge(a, b, "k", nil); err != nil { // rev 3
		t.Fatal(err)
	}
	if err := s.DeleteNode(a); err != nil { // rev 4 node + rev 5 cascade edge
		t.Fatal(err)
	}
	edgeID := a + "->" + b + "@k"
	touching := s.EventsTouching(edgeID)
	if len(touching) != 2 || touching[1].Op != "delete" {
		t.Fatalf("cascade events: %+v", touching)
	}
	// cascade shares the parent delete's transaction
	nodeEv := s.EventsTouching(a)
	if len(nodeEv) != 2 || nodeEv[1].Transaction != touching[1].Transaction {
		t.Fatalf("cascade tx mismatch: %+v vs %+v", nodeEv, touching)
	}
	if got := s.GetEventsByTransaction(nodeEv[1].Transaction); len(got) != 2 {
		t.Fatalf("tx group: %+v", got)
	}
}

// metaStore mirrors extensions/cti's unexported interface (same signatures)
// so the cti type assertion s.(metaStore) succeeds for *Storage.
type metaStore interface {
	AddNodeWithMeta(label string, props map[string]any, source, transaction string) (string, error)
	AddEdgeWithMeta(from, to, kind string, props map[string]any, source, transaction string) error
	UpdateNodeWithMeta(id string, props map[string]any, source, transaction string) error
	UpdateEdgeWithMeta(id string, props map[string]any, source, transaction string) error
}

var _ metaStore = (*Storage)(nil)

// syncMetaStore mirrors cmd's sync write path (metaStore + deletes): bolt
// must satisfy it fully or sync silently falls back to sourceless writes.
type syncMetaStore interface {
	metaStore
	DeleteNodeWithMeta(id, source, transaction string) error
	DeleteEdgeWithMeta(from, to, kind, source, transaction string) error
}

var _ syncMetaStore = (*Storage)(nil)

func TestBoltWithMeta(t *testing.T) {
	s := openTest(t)

	a, err := s.AddNodeWithMeta("person", map[string]any{"v": 1}, "feed1", "tx-1")
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.AddNodeWithMeta("person", map[string]any{"v": 1}, "feed1", "tx-1")
	if err != nil {
		t.Fatal(err)
	}
	edgeID := a + "->" + b + "@knows"
	if err := s.AddEdgeWithMeta(a, b, "knows", map[string]any{"w": 1}, "feed1", "tx-1"); err != nil {
		t.Fatal(err)
	}

	// snapshots carry source + transaction (visible via GetNode: latest history)
	h := s.GetNodeHistory(a)
	if len(h) != 1 || h[0].Source != "feed1" || h[0].Transaction != "tx-1" {
		t.Fatalf("node snapshot meta: %+v", h)
	}
	if n, _ := s.GetNode(a); n.History[len(n.History)-1].Source != "feed1" {
		t.Fatal("GetNode should return latest source via history")
	}
	// events carry them too; one tx groups create+create
	if got := s.GetEventsByTransaction("tx-1"); len(got) != 3 {
		t.Fatalf("tx-1 group: %+v", got)
	} else if got[2].Source != "feed1" || got[2].ElementType != "edge" || got[2].From != a {
		t.Fatalf("edge create event: %+v", got[2])
	}

	// empty transaction auto-generates a UUID shared by snapshot + event
	c, err := s.AddNodeWithMeta("org", nil, "feed2", "")
	if err != nil {
		t.Fatal(err)
	}
	ch := s.GetNodeHistory(c)
	if len(ch) != 1 || ch[0].Transaction == "" || ch[0].Source != "feed2" {
		t.Fatalf("auto-tx snapshot: %+v", ch)
	}
	cev := s.EventsTouching(c)
	if len(cev) != 1 || cev[0].Transaction != ch[0].Transaction {
		t.Fatalf("auto-tx event mismatch: %+v vs %+v", cev, ch)
	}

	// updates stamp source/transaction on snapshot + event
	if err := s.UpdateNodeWithMeta(a, map[string]any{"v": 2}, "feed2", "tx-2"); err != nil {
		t.Fatal(err)
	}
	if h := s.GetNodeHistory(a); len(h) != 2 || h[1].Source != "feed2" || h[1].Transaction != "tx-2" {
		t.Fatalf("node update snapshot: %+v", h)
	}
	if err := s.UpdateEdgeWithMeta(edgeID, map[string]any{"w": 2}, "feed2", "tx-2"); err != nil {
		t.Fatal(err)
	}
	if eh := s.GetEdgeHistory(edgeID); len(eh) != 2 || eh[1].Source != "feed2" || eh[1].Transaction != "tx-2" {
		t.Fatalf("edge update snapshot: %+v", eh)
	}
	if got := s.GetEventsByTransaction("tx-2"); len(got) != 2 {
		t.Fatalf("tx-2 group: %+v", got)
	} else if got[0].Op != "update" || got[0].Source != "feed2" || got[1].ElementType != "edge" {
		t.Fatalf("tx-2 events: %+v", got)
	}

	// plain writes still auto-generate (no regression from the refactor)
	d, err := s.AddNode("x", nil)
	if err != nil {
		t.Fatal(err)
	}
	if ev := s.EventsTouching(d); len(ev) != 1 || ev[0].Transaction == "" {
		t.Fatalf("plain AddNode event: %+v", ev)
	}

	// errors match inmem
	if err := s.UpdateNodeWithMeta("missing", nil, "s", "t"); err == nil {
		t.Fatal("UpdateNodeWithMeta missing should fail")
	}
	if err := s.AddEdgeWithMeta(a, "missing", "k", nil, "s", "t"); err == nil {
		t.Fatal("AddEdgeWithMeta bad target should fail")
	}
}
