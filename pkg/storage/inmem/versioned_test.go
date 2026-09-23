// version: tests — append-only node versioning (ADR 0002).
package inmem_test

import (
	"encoding/gob"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/aprksy/knitknot/pkg/ports/types"
	"github.com/aprksy/knitknot/pkg/storage/file"
	"github.com/aprksy/knitknot/pkg/storage/inmem"
)

var _ = Describe("VersionedStorage (node-only)", func() {
	var storage *inmem.Storage

	BeforeEach(func() {
		storage = inmem.New()
	})

	It("AppendOnUpdate: history grows, GetNode sees latest", func() {
		id, err := storage.AddNode("User", map[string]any{"name": "Alice"})
		Expect(err).NotTo(HaveOccurred())
		Expect(storage.UpdateNode(id, map[string]any{"name": "Alice2"})).To(Succeed())

		node, ok := storage.GetNode(id)
		Expect(ok).To(BeTrue())
		Expect(node.Props["name"]).To(Equal("Alice2"))
		Expect(node.History).To(HaveLen(2))
		Expect(node.CreatedRev).To(Equal(int64(1)))

		h := storage.GetNodeHistory(id)
		Expect(h).To(HaveLen(2))
		Expect(h[0].Props["name"]).To(Equal("Alice"))
		Expect(h[1].Props["name"]).To(Equal("Alice2"))
	})

	It("GetNodeAtPastFutureMissing", func() {
		id, err := storage.AddNode("User", map[string]any{"v": 1})
		Expect(err).NotTo(HaveOccurred())
		time.Sleep(2 * time.Millisecond)
		Expect(storage.UpdateNode(id, map[string]any{"v": 2})).To(Succeed())

		h := storage.GetNodeHistory(id)
		t0, t1 := h[0].EventTime, h[1].EventTime

		at0, ok := storage.GetNodeAt(id, t0)
		Expect(ok).To(BeTrue())
		Expect(at0.Props["v"]).To(Equal(1))

		at1, ok := storage.GetNodeAt(id, t1)
		Expect(ok).To(BeTrue())
		Expect(at1.Props["v"]).To(Equal(2))

		_, ok = storage.GetNodeAt(id, time.Time{})
		Expect(ok).To(BeFalse())

		_, ok = storage.GetNodeAt("missing", t1)
		Expect(ok).To(BeFalse())
	})

	It("TombstoneSemantics: delete keeps history, hides past-delete views", func() {
		id, err := storage.AddNode("User", map[string]any{"v": 1})
		Expect(err).NotTo(HaveOccurred())
		h0 := storage.GetNodeHistory(id)
		time.Sleep(2 * time.Millisecond)
		Expect(storage.DeleteNode(id)).To(Succeed())

		node, ok := storage.GetNode(id)
		Expect(ok).To(BeTrue())
		Expect(node.DeletedAt).NotTo(BeNil())

		live, ok := storage.GetNodeAt(id, h0[0].EventTime)
		Expect(ok).To(BeTrue())
		Expect(live.Props["v"]).To(Equal(1))

		h := storage.GetNodeHistory(id)
		Expect(h).To(HaveLen(2))
		_, ok = storage.GetNodeAt(id, h[1].EventTime)
		Expect(ok).To(BeFalse())
	})

	It("EventLogOrdering: create/update/delete are Rev 1,2,3", func() {
		id, err := storage.AddNode("User", nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(storage.UpdateNode(id, map[string]any{"v": 2})).To(Succeed())
		Expect(storage.DeleteNode(id)).To(Succeed())

		all := storage.GetEvents(0, 0)
		Expect(all).To(HaveLen(3))
		Expect(all[0].Rev).To(Equal(int64(1)))
		Expect(all[1].Rev).To(Equal(int64(2)))
		Expect(all[2].Rev).To(Equal(int64(3)))
		Expect(all[0].Op).To(Equal("create"))
		Expect(all[1].Op).To(Equal("update"))
		Expect(all[2].Op).To(Equal("delete"))

		last2 := storage.GetEvents(2, 3)
		Expect(last2).To(HaveLen(2))
		Expect(last2[0].Rev).To(Equal(int64(2)))
	})

	It("TransactionGrouping: same tx groups both creates", func() {
		_, err := storage.AddNodeWithMeta("A", nil, "", "tx-1")
		Expect(err).NotTo(HaveOccurred())
		_, err = storage.AddNodeWithMeta("B", nil, "", "tx-1")
		Expect(err).NotTo(HaveOccurred())

		evs := storage.GetEventsByTransaction("tx-1")
		Expect(evs).To(HaveLen(2))
		Expect(storage.GetEventsByTransaction("nope")).To(BeEmpty())
	})

	It("EventsTouching: only the element's own events", func() {
		id1, err := storage.AddNode("A", nil)
		Expect(err).NotTo(HaveOccurred())
		_, err = storage.AddNode("B", nil)
		Expect(err).NotTo(HaveOccurred())
		_, err = storage.AddNode("C", nil)
		Expect(err).NotTo(HaveOccurred())

		evs := storage.EventsTouching(id1)
		Expect(evs).To(HaveLen(1))
		Expect(evs[0].ElementID).To(Equal(id1))
	})

	It("LiveOnlyEnumerate: tombstoned nodes are hidden from enumeration, visible via GetNode", func() {
		id, err := storage.AddNode("User", map[string]any{"v": 1})
		Expect(err).NotTo(HaveOccurred())
		Expect(storage.DeleteNode(id)).To(Succeed())

		// ADR: GetNode returns the tombstone for history access...
		_, ok := storage.GetNode(id)
		Expect(ok).To(BeTrue())

		// ...but enumeration paths (what the query engine / REPL / export
		// use) must stay live-only, restoring pre-ADR behavior.
		Expect(storage.GetAllNodes()).To(BeEmpty())
		Expect(storage.GetNodesByLabel("User")).To(BeEmpty())
		Expect(storage.GetNodesIn("x")).To(BeEmpty())
	})

	It("LegacyMigration: v0.2 nodes gain one synthetic snapshot", func() {
		tmpDir, err := os.MkdirTemp("", "knitknot-legacy-*")
		Expect(err).NotTo(HaveOccurred())
		defer os.RemoveAll(tmpDir)
		filename := filepath.Join(tmpDir, "legacy.gob")

		// Build a v0.2-style file: nodes with no History field populated.
		f, err := os.Create(filename)
		Expect(err).NotTo(HaveOccurred())
		saved := &file.SavedGraph{
			Version: "knitknot/v0.2",
			Nodes: map[string]*types.Node{
				"n9": {ID: "n9", Label: "User", Props: map[string]any{"name": "Old"}},
			},
			Edges: map[string]*types.Edge{},
			Verbs: map[string]types.Verb{},
		}
		Expect(gob.NewEncoder(f).Encode(saved)).To(Succeed())
		Expect(f.Close()).To(Succeed())

		fresh := inmem.New()
		Expect(fresh.Load(filename, nil)).To(Succeed())

		node, ok := fresh.GetNode("n9")
		Expect(ok).To(BeTrue())
		Expect(node.History).To(HaveLen(1))
		Expect(node.History[0].Source).To(Equal("legacy"))
		Expect(node.History[0].Rev).To(Equal(int64(1)))
		Expect(node.History[0].Deleted).To(BeFalse())
		Expect(node.Props["name"]).To(Equal("Old"))
	})
})
