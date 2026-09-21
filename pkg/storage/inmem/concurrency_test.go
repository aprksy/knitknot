// M4 + H4 regression: concurrent subgraph access must not race (verified with -race).
package inmem_test

import (
	"fmt"
	"runtime"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/aprksy/knitknot/pkg/storage/inmem"
)

var _ = Describe("Concurrency", func() {
	var storage *inmem.Storage

	BeforeEach(func() {
		storage = inmem.New()
	})

	// M4 regression: concurrent AddToSubgraph on the SAME node pointer raced
	// before the fix (unsynchronized map write on n.Subgraphs).
	Context("M4: concurrent AddToSubgraph on shared node", func() {
		It("does not race with 100 goroutines plus RemoveFromSubgraph", func() {
			id, err := storage.AddNode("User", nil)
			Expect(err).NotTo(HaveOccurred())

			n, ok := storage.GetNode(id)
			Expect(ok).To(BeTrue())

			var wg sync.WaitGroup
			for i := 0; i < 100; i++ {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					defer GinkgoRecover()
					storage.AddToSubgraph(n, fmt.Sprintf("sg%d", i), "desc")
				}(i)
			}
			wg.Wait()

			Expect(n.Subgraphs).To(HaveLen(100))

			storage.RemoveFromSubgraph(n, "sg0")
			Expect(n.Subgraphs).NotTo(HaveKey("sg0"))
			Expect(n.Subgraphs).To(HaveLen(99))
		})
	})

	// H4 regression: concurrent GetEdgesIn raced before the fix in the
	// auto-inherit branch (inmem.go lines 198-215) which mutated
	// e.Subgraphs under RLock. The fix copies instead of mutating.
	Context("H4: concurrent GetEdgesIn auto-inherit branch", func() {
		It("does not race under tight concurrent read loop", func() {
			a, err := storage.AddNode("A", nil)
			Expect(err).NotTo(HaveOccurred())
			b, err := storage.AddNode("B", nil)
			Expect(err).NotTo(HaveOccurred())

			na, ok := storage.GetNode(a)
			Expect(ok).To(BeTrue())
			nb, ok := storage.GetNode(b)
			Expect(ok).To(BeTrue())

			// Both ends in sgX, but the edge itself has no sgX entry,
			// so GetEdgesIn("sgX") takes the auto-inherit branch.
			storage.AddToSubgraph(na, "sgX", "desc")
			storage.AddToSubgraph(nb, "sgX", "desc")
			Expect(storage.AddEdge(a, b, "rel", nil)).To(Succeed())

			goroutines := runtime.NumCPU() * 4
			var wg sync.WaitGroup
			for g := 0; g < goroutines; g++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					defer GinkgoRecover()
					for i := 0; i < 500; i++ {
						_ = storage.GetEdgesIn("sgX")
					}
				}()
			}
			wg.Wait()

			// Sanity: auto-inherit still resolves the edge.
			Expect(storage.GetEdgesIn("sgX")).To(HaveLen(1))
		})
	})
})
