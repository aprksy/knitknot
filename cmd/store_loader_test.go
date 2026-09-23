package cmd

import (
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/aprksy/knitknot/pkg/graph"
	"github.com/aprksy/knitknot/pkg/storage/inmem"
)

var _ = Describe("resolveStoreBackend", func() {
	var saved string

	BeforeEach(func() {
		saved = storeFlag
		storeFlag = ""
	})

	AfterEach(func() {
		storeFlag = saved
	})

	It("returns the same engine when --store is unset", func() {
		engine := graph.NewGraphEngine(inmem.New())
		got, err := resolveStoreBackend(engine) // store: wire
		Expect(err).NotTo(HaveOccurred())
		Expect(got == engine).To(BeTrue())
	})

	It("opens an existing gob file via the selector", func() {
		src := graph.NewGraphEngine(inmem.New())
		_, err := src.AddNode("Person", map[string]any{"name": "Alice"})
		Expect(err).NotTo(HaveOccurred())
		path := filepath.Join(GinkgoT().TempDir(), "s.gob")
		Expect(src.Storage().(*inmem.Storage).Save(path, src)).To(Succeed())

		storeFlag = "gob:" + path
		engine := graph.NewGraphEngine(inmem.New())
		got, err := resolveStoreBackend(engine) // store: wire
		Expect(err).NotTo(HaveOccurred())
		Expect(got == engine).To(BeFalse())
		Expect(got.Storage().GetAllNodes()).To(HaveLen(1))
	})

	It("returns a fresh empty engine for a missing gob file", func() {
		storeFlag = "gob:" + filepath.Join(GinkgoT().TempDir(), "nonexistent.gob")
		engine := graph.NewGraphEngine(inmem.New())
		got, err := resolveStoreBackend(engine) // store: wire
		Expect(err).NotTo(HaveOccurred())
		Expect(got == engine).To(BeFalse())
		Expect(got.Storage().GetAllNodes()).To(BeEmpty())
	})

	It("errors on an unknown scheme", func() {
		storeFlag = "sqlite:foo.db"
		engine := graph.NewGraphEngine(inmem.New())
		_, err := resolveStoreBackend(engine) // store: wire
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("sqlite"))
	})
})
