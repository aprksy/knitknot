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

			_, ok := storage.GetNode(a)
			Expect(ok).To(BeFalse())

			Expect(storage.GetEdgesFrom(a)).To(BeEmpty())
			Expect(storage.GetEdgesTo(a)).To(BeEmpty())
			Expect(storage.GetAllEdges()).To(HaveLen(1))

			remaining := storage.GetAllEdges()
			Expect(remaining[0].From).To(Equal(b))
			Expect(remaining[0].To).To(Equal(c))
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

			_, ok := storage.GetNode(lonely)
			Expect(ok).To(BeFalse())
		})

		It("returns an error for a missing id", func() {
			Expect(storage.DeleteNode("missing")).NotTo(Succeed())
		})
	})
})
