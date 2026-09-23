// M3 regression: DeleteNode must cascade to incident edges.
package inmem_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/aprksy/knitknot/pkg/storage/inmem"
)

var _ = Describe("DeleteNode cascade", func() {
	var storage *inmem.Storage

	BeforeEach(func() {
		storage = inmem.New()
	})

	// M3 regression: DeleteNode(A) must remove every edge incident to A
	// while leaving unrelated edges intact.
	Context("M3: DeleteNode cascades to incident edges", func() {
		It("removes A->B and A->C but keeps B->C", func() {
			a, err := storage.AddNode("A", nil)
			Expect(err).NotTo(HaveOccurred())
			b, err := storage.AddNode("B", nil)
			Expect(err).NotTo(HaveOccurred())
			c, err := storage.AddNode("C", nil)
			Expect(err).NotTo(HaveOccurred())

			Expect(storage.AddEdge(a, b, "rel", nil)).To(Succeed())
			Expect(storage.AddEdge(b, c, "rel", nil)).To(Succeed())
			Expect(storage.AddEdge(a, c, "rel", nil)).To(Succeed())
			Expect(storage.GetAllEdges()).To(HaveLen(3))

			Expect(storage.DeleteNode(a)).To(Succeed())

			// version: tombstone — node stays findable with DeletedAt set.
			tomb, ok := storage.GetNode(a)
			Expect(ok).To(BeTrue())
			Expect(tomb.DeletedAt).NotTo(BeNil())

			// version: cascade-tombstone — incident edges are tombstones, not removals.
			Expect(storage.GetEdgesFrom(a)).To(BeEmpty())
			Expect(storage.GetEdgesTo(a)).To(BeEmpty())
			Expect(storage.GetAllEdges()).To(HaveLen(1))

			remaining := storage.GetAllEdges()
			Expect(remaining[0].From).To(Equal(b))
			Expect(remaining[0].To).To(Equal(c))

			edgeAB, ok := storage.GetEdge(a + "->" + b + "@rel")
			Expect(ok).To(BeTrue())
			Expect(edgeAB.DeletedAt).NotTo(BeNil())
			edgeAC, ok := storage.GetEdge(a + "->" + c + "@rel")
			Expect(ok).To(BeTrue())
			Expect(edgeAC.DeletedAt).NotTo(BeNil())
			Expect(storage.GetEdgeHistory(a + "->" + b + "@rel")).To(HaveLen(2))
		})

		It("deletes a node with no incident edges without side effects", func() {
			a, err := storage.AddNode("A", nil)
			Expect(err).NotTo(HaveOccurred())
			b, err := storage.AddNode("B", nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(storage.AddEdge(a, b, "rel", nil)).To(Succeed())

			lonely, err := storage.AddNode("Lonely", nil)
			Expect(err).NotTo(HaveOccurred())

			before := len(storage.GetAllEdges())
			Expect(storage.DeleteNode(lonely)).To(Succeed())
			Expect(storage.GetAllEdges()).To(HaveLen(before))

			// version: tombstone — node stays findable with DeletedAt set.
			tomb, ok := storage.GetNode(lonely)
			Expect(ok).To(BeTrue())
			Expect(tomb.DeletedAt).NotTo(BeNil())
		})

		It("returns an error for a missing id", func() {
			Expect(storage.DeleteNode("missing")).NotTo(Succeed())
		})

		// version: cascade-tombstone — cascade shares the parent's transaction.
		It("DeleteNodeWithMeta cascades incident edges as tombstones with shared transaction", func() {
			a, err := storage.AddNode("A", nil)
			Expect(err).NotTo(HaveOccurred())
			b, err := storage.AddNode("B", nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(storage.AddEdge(a, b, "rel", nil)).To(Succeed())

			txid := "tx-cascade-1"
			Expect(storage.DeleteNodeWithMeta(a, "test", txid)).To(Succeed())

			events := storage.GetEventsByTransaction(txid)
			Expect(events).To(HaveLen(2))
			Expect(events[0].Op).To(Equal("delete"))
			Expect(events[0].ElementType).To(Equal("node"))
			Expect(events[0].ElementID).To(Equal(a))
			Expect(events[1].Op).To(Equal("delete"))
			Expect(events[1].ElementType).To(Equal("edge"))
			Expect(events[1].ElementID).To(Equal(a + "->" + b + "@rel"))
			Expect(events[1].From).To(Equal(a))
			Expect(events[1].To).To(Equal(b))
			Expect(events[1].Kind).To(Equal("rel"))
			Expect(events[1].Source).To(Equal("test"))
			Expect(events[1].Transaction).To(Equal(txid))
		})
	})
})
