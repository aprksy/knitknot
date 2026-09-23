// version: edge — append-only edge versioning tests (ADR 0002 follow-up).
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

var _ = Describe("VersionedStorage (edge)", func() {
	var storage *inmem.Storage
	var fromID, toID, edgeID string

	BeforeEach(func() {
		storage = inmem.New()
		var err error
		fromID, err = storage.AddNode("User", nil)
		Expect(err).NotTo(HaveOccurred())
		toID, err = storage.AddNode("Skill", nil)
		Expect(err).NotTo(HaveOccurred())
		edgeID = fromID + "->" + toID + "@has_skill"
	})

	It("AppendOnUpdateEdge: history grows, GetEdge sees latest", func() {
		Expect(storage.AddEdge(fromID, toID, "has_skill", map[string]any{"level": 1})).To(Succeed())
		Expect(storage.UpdateEdge(edgeID, map[string]any{"level": 2})).To(Succeed())

		edge, ok := storage.GetEdge(edgeID)
		Expect(ok).To(BeTrue())
		Expect(edge.Props["level"]).To(Equal(2))
		Expect(edge.History).To(HaveLen(2))
		Expect(edge.CreatedRev).To(Equal(int64(3)))

		h := storage.GetEdgeHistory(edgeID)
		Expect(h).To(HaveLen(2))
		Expect(h[0].Props["level"]).To(Equal(1))
		Expect(h[1].Props["level"]).To(Equal(2))
	})

	It("GetEdgeAtPastFutureMissing", func() {
		Expect(storage.AddEdge(fromID, toID, "has_skill", map[string]any{"v": 1})).To(Succeed())
		time.Sleep(2 * time.Millisecond)
		Expect(storage.UpdateEdge(edgeID, map[string]any{"v": 2})).To(Succeed())

		h := storage.GetEdgeHistory(edgeID)
		t0, t1 := h[0].EventTime, h[1].EventTime

		at0, ok := storage.GetEdgeAt(edgeID, t0)
		Expect(ok).To(BeTrue())
		Expect(at0.Props["v"]).To(Equal(1))

		at1, ok := storage.GetEdgeAt(edgeID, t1)
		Expect(ok).To(BeTrue())
		Expect(at1.Props["v"]).To(Equal(2))

		_, ok = storage.GetEdgeAt(edgeID, time.Time{})
		Expect(ok).To(BeFalse())

		_, ok = storage.GetEdgeAt("missing", t1)
		Expect(ok).To(BeFalse())
	})

	It("EdgeTombstoneSemantics: delete keeps history, hides past-delete views", func() {
		Expect(storage.AddEdge(fromID, toID, "has_skill", map[string]any{"v": 1})).To(Succeed())
		h0 := storage.GetEdgeHistory(edgeID)
		time.Sleep(2 * time.Millisecond)
		Expect(storage.DeleteEdge(fromID, toID, "has_skill")).To(Succeed())

		edge, ok := storage.GetEdge(edgeID)
		Expect(ok).To(BeTrue())
		Expect(edge.DeletedAt).NotTo(BeNil())

		live, ok := storage.GetEdgeAt(edgeID, h0[0].EventTime)
		Expect(ok).To(BeTrue())
		Expect(live.Props["v"]).To(Equal(1))

		h := storage.GetEdgeHistory(edgeID)
		Expect(h).To(HaveLen(2))
		_, ok = storage.GetEdgeAt(edgeID, h[1].EventTime)
		Expect(ok).To(BeFalse())
	})

	It("EdgeLiveOnlyEnumeration: tombstoned edges hidden, visible via GetEdge", func() {
		to2, err := storage.AddNode("Skill", nil)
		Expect(err).NotTo(HaveOccurred())
		to3, err := storage.AddNode("Skill", nil)
		Expect(err).NotTo(HaveOccurred())

		Expect(storage.AddEdge(fromID, toID, "has_skill", nil)).To(Succeed())
		Expect(storage.AddEdge(fromID, to2, "has_skill", nil)).To(Succeed())
		Expect(storage.AddEdge(fromID, to3, "knows", nil)).To(Succeed())
		Expect(storage.DeleteEdge(fromID, toID, "has_skill")).To(Succeed())

		Expect(storage.GetAllEdges()).To(HaveLen(2))
		Expect(storage.GetEdgesFrom(fromID)).To(HaveLen(2))
		Expect(storage.GetEdgesTo(toID)).To(BeEmpty())
		Expect(storage.GetEdgesByKind("has_skill")).To(HaveLen(1))

		// Tombstone stays findable via GetEdge with DeletedAt set.
		tomb, ok := storage.GetEdge(edgeID)
		Expect(ok).To(BeTrue())
		Expect(tomb.DeletedAt).NotTo(BeNil())
	})

	It("EdgeLiveOnlyEnumeration via subgraph: GetEdgesIn excludes tombstones", func() {
		storage.AddToSubgraph(mustGetNode(storage, fromID), "sg", "d")
		storage.AddToSubgraph(mustGetNode(storage, toID), "sg", "d")
		Expect(storage.AddEdge(fromID, toID, "has_skill", nil)).To(Succeed())
		Expect(storage.GetEdgesIn("sg")).NotTo(BeEmpty())
		Expect(storage.DeleteEdge(fromID, toID, "has_skill")).To(Succeed())
		Expect(storage.GetEdgesIn("sg")).To(BeEmpty())

		tomb, ok := storage.GetEdge(edgeID)
		Expect(ok).To(BeTrue())
		Expect(tomb.DeletedAt).NotTo(BeNil())
	})

	It("EdgeEventLog: create/update/delete carry From/To/Kind", func() {
		Expect(storage.AddEdge(fromID, toID, "has_skill", nil)).To(Succeed())
		Expect(storage.UpdateEdge(edgeID, map[string]any{"v": 2})).To(Succeed())
		Expect(storage.DeleteEdge(fromID, toID, "has_skill")).To(Succeed())

		evs := storage.EventsTouching(edgeID)
		Expect(evs).To(HaveLen(3))
		Expect(evs[0].Op).To(Equal("create"))
		Expect(evs[1].Op).To(Equal("update"))
		Expect(evs[2].Op).To(Equal("delete"))
		for _, e := range evs {
			Expect(e.ElementType).To(Equal("edge"))
			Expect(e.From).To(Equal(fromID))
			Expect(e.To).To(Equal(toID))
			Expect(e.Kind).To(Equal("has_skill"))
		}
	})

	It("EdgeTransactionGrouping: same tx groups both creates", func() {
		Expect(storage.AddEdgeWithMeta(fromID, toID, "rel1", nil, "", "tx-e")).To(Succeed())
		Expect(storage.AddEdgeWithMeta(fromID, toID, "rel2", nil, "", "tx-e")).To(Succeed())

		evs := storage.GetEventsByTransaction("tx-e")
		Expect(evs).To(HaveLen(2))
	})

	It("EdgeEventsTouching: only that edge's events", func() {
		Expect(storage.AddEdge(fromID, toID, "rel1", nil)).To(Succeed())
		Expect(storage.AddEdge(fromID, toID, "rel2", nil)).To(Succeed())

		evs := storage.EventsTouching(fromID + "->" + toID + "@rel1")
		Expect(evs).To(HaveLen(1))
		Expect(evs[0].Kind).To(Equal("rel1"))
	})

	It("EdgeLegacyMigration: v0.3 edges gain one synthetic snapshot", func() {
		tmpDir, err := os.MkdirTemp("", "knitknot-legacy-edge-*")
		Expect(err).NotTo(HaveOccurred())
		defer os.RemoveAll(tmpDir)
		filename := filepath.Join(tmpDir, "legacy.gob")

		f, err := os.Create(filename)
		Expect(err).NotTo(HaveOccurred())
		saved := &file.SavedGraph{
			Version: "knitknot/v0.3",
			Nodes: map[string]*types.Node{
				"n1": {ID: "n1", Label: "User", Props: map[string]any{},
					History: []types.Snapshot{{Rev: 1, Source: "legacy", Props: map[string]any{}}}, CreatedRev: 1},
				"n2": {ID: "n2", Label: "Skill", Props: map[string]any{},
					History: []types.Snapshot{{Rev: 2, Source: "legacy", Props: map[string]any{}}}, CreatedRev: 2},
			},
			Edges: map[string]*types.Edge{
				"n1->n2@rel": {ID: "n1->n2@rel", From: "n1", To: "n2", Kind: "rel", Props: map[string]any{"w": 1}},
			},
			Verbs: map[string]types.Verb{},
		}
		Expect(gob.NewEncoder(f).Encode(saved)).To(Succeed())
		Expect(f.Close()).To(Succeed())

		fresh := inmem.New()
		Expect(fresh.Load(filename, nil)).To(Succeed())

		edge, ok := fresh.GetEdge("n1->n2@rel")
		Expect(ok).To(BeTrue())
		Expect(edge.History).To(HaveLen(1))
		Expect(edge.History[0].Source).To(Equal("legacy"))
		Expect(edge.History[0].Rev).To(Equal(int64(1)))
		Expect(edge.History[0].Deleted).To(BeFalse())
		Expect(edge.Props["w"]).To(Equal(1))
	})
})

func mustGetNode(s *inmem.Storage, id string) *types.Node {
	n, ok := s.GetNode(id)
	if !ok {
		panic("node not found: " + id)
	}
	return n
}
