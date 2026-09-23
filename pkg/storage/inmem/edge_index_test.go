// perf: edge-index — index behavior for GetEdgesFrom/To/ByKind.
package inmem_test

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/aprksy/knitknot/pkg/storage/inmem"
)

var _ = Describe("Edge indexes", func() {
	var storage *inmem.Storage

	BeforeEach(func() {
		storage = inmem.New()
	})

	mustNode := func(label string) string {
		id, err := storage.AddNode(label, nil)
		Expect(err).NotTo(HaveOccurred())
		return id
	}

	Context("EdgesByFromIndex", func() {
		It("returns edges from A in any order, none from B", func() {
			a := mustNode("A")
			b := mustNode("B")
			c := mustNode("C")
			d := mustNode("D")

			Expect(storage.AddEdge(a, b, "rel", nil)).To(Succeed())
			Expect(storage.AddEdge(a, c, "rel", nil)).To(Succeed())
			Expect(storage.AddEdge(a, d, "rel", nil)).To(Succeed())

			from := storage.GetEdgesFrom(a)
			Expect(from).To(HaveLen(3))
			tos := []string{from[0].To, from[1].To, from[2].To}
			Expect(tos).To(ConsistOf(b, c, d))

			Expect(storage.GetEdgesFrom(b)).To(BeEmpty())
		})
	})

	Context("EdgesByToIndex", func() {
		It("returns edges to B in any order, none to A", func() {
			a := mustNode("A")
			b := mustNode("B")
			c := mustNode("C")
			d := mustNode("D")

			Expect(storage.AddEdge(a, b, "rel", nil)).To(Succeed())
			Expect(storage.AddEdge(c, b, "rel", nil)).To(Succeed())
			Expect(storage.AddEdge(d, b, "rel", nil)).To(Succeed())

			to := storage.GetEdgesTo(b)
			Expect(to).To(HaveLen(3))
			froms := []string{to[0].From, to[1].From, to[2].From}
			Expect(froms).To(ConsistOf(a, c, d))

			Expect(storage.GetEdgesTo(a)).To(BeEmpty())
		})
	})

	Context("EdgesByKindIndex", func() {
		It("groups edges by kind", func() {
			a := mustNode("A")
			b := mustNode("B")

			Expect(storage.AddEdge(a, b, "rel", nil)).To(Succeed())
			Expect(storage.AddEdge(a, b, "uses", nil)).To(Succeed())
			c := mustNode("C")
			Expect(storage.AddEdge(a, c, "rel", nil)).To(Succeed())

			Expect(storage.GetEdgesByKind("rel")).To(HaveLen(2))
			Expect(storage.GetEdgesByKind("uses")).To(HaveLen(1))
			Expect(storage.GetEdgesByKind("missing")).To(BeEmpty())
		})
	})

	Context("DeleteEdge tombstone keeps edge in primary map and indexes", func() {
		It("hides the tombstoned edge from enumeration but keeps history access", func() {
			a := mustNode("A")
			b := mustNode("B")
			c := mustNode("C")

			Expect(storage.AddEdge(a, b, "rel", nil)).To(Succeed())
			Expect(storage.AddEdge(a, c, "rel", nil)).To(Succeed())
			Expect(storage.AddEdge(b, c, "uses", nil)).To(Succeed())

			Expect(storage.DeleteEdge(a, b, "rel")).To(Succeed())

			// version: tombstone — edge stays in the primary map with DeletedAt set.
			tomb, ok := storage.GetEdge(a + "->" + b + "@rel")
			Expect(ok).To(BeTrue())
			Expect(tomb.DeletedAt).NotTo(BeNil())

			// version: live-only enumeration — tombstone hidden from all three
			// indexed getters even though it remains in the index buckets.
			Expect(storage.GetEdgesFrom(a)).To(HaveLen(1))
			Expect(storage.GetEdgesTo(b)).To(BeEmpty())
			Expect(storage.GetEdgesByKind("rel")).To(HaveLen(1))
			Expect(storage.GetEdgesByKind("uses")).To(HaveLen(1))
			Expect(storage.GetAllEdges()).To(HaveLen(2))
		})
	})

	Context("TombstoneKeepsEdgesInIndexes", func() {
		It("cascade-tombstoned edges stay history-accessible but enumerate empty", func() {
			a := mustNode("A")
			b := mustNode("B")

			Expect(storage.AddEdge(a, b, "rel", nil)).To(Succeed())
			Expect(storage.DeleteNode(a)).To(Succeed())

			// Still in the primary map with DeletedAt set (and hence still in
			// the index buckets, which mirror the primary map).
			tomb, ok := storage.GetEdge(a + "->" + b + "@rel")
			Expect(ok).To(BeTrue())
			Expect(tomb.DeletedAt).NotTo(BeNil())

			// Live-only enumeration hides it.
			Expect(storage.GetEdgesFrom(a)).To(BeEmpty())
			Expect(storage.GetEdgesTo(b)).To(BeEmpty())
			Expect(storage.GetEdgesByKind("rel")).To(BeEmpty())
		})
	})

	Context("Load rebuilds edge indexes", func() {
		It("serves indexed reads after a Save/Load round-trip", func() {
			a := mustNode("A")
			b := mustNode("B")
			c := mustNode("C")

			Expect(storage.AddEdge(a, b, "rel", nil)).To(Succeed())
			Expect(storage.AddEdge(a, c, "uses", nil)).To(Succeed())

			f, err := os.CreateTemp("", "edgeidx*.gob")
			Expect(err).NotTo(HaveOccurred())
			path := f.Name()
			Expect(f.Close()).To(Succeed())
			defer func() { _ = os.Remove(path) }()

			Expect(storage.Save(path, nil)).To(Succeed())

			restored := inmem.New()
			Expect(restored.Load(path, nil)).To(Succeed())

			Expect(restored.GetEdgesFrom(a)).To(HaveLen(2))
			Expect(restored.GetEdgesTo(b)).To(HaveLen(1))
			Expect(restored.GetEdgesByKind("rel")).To(HaveLen(1))
			Expect(restored.GetEdgesByKind("uses")).To(HaveLen(1))
		})
	})
})
