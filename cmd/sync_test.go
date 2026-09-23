package cmd

import (
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/aprksy/knitknot/pkg/graph"
	"github.com/aprksy/knitknot/pkg/ports/types"
	"github.com/aprksy/knitknot/pkg/storage/inmem"
)

// store: sync — helper: build a gob file via setup, then persist it.
func mustWriteGob(path string, setup func(*graph.GraphEngine)) {
	eng := graph.NewGraphEngine(inmem.New())
	setup(eng)
	Expect(SaveGraph(eng, path)).To(Succeed())
}

// store: sync — helper: first node carrying props[key] == val, or nil.
func findSyncNode(eng *graph.GraphEngine, key string, val any) *types.Node {
	for _, n := range eng.Storage().GetAllNodes() {
		if n.Props[key] == val {
			return n
		}
	}
	return nil
}

func runSyncCmd(args ...string) error {
	c := newSyncCmd()
	c.SetArgs(args)
	return c.Execute()
}

var _ = Describe("sync command", func() {
	It("returns a usage error when --from is missing", func() {
		err := runSyncCmd("--to", "gob:b.gob")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("--from"))
	})

	It("returns a usage error when --to is missing", func() {
		err := runSyncCmd("--from", "gob:a.gob")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("--to"))
	})

	It("merges nodes and edges from --from into --to (gob -> gob)", func() {
		dir := GinkgoT().TempDir()
		a := filepath.Join(dir, "a.gob")
		b := filepath.Join(dir, "b.gob")
		mustWriteGob(a, func(eng *graph.GraphEngine) {
			x, err := eng.AddNode("person", map[string]any{"name": "a-x"})
			Expect(err).NotTo(HaveOccurred())
			y, err := eng.AddNode("person", map[string]any{"name": "a-y"})
			Expect(err).NotTo(HaveOccurred())
			Expect(eng.AddEdge(x, y, "knows", map[string]any{"since": "2024"})).To(Succeed())
		})
		mustWriteGob(b, func(eng *graph.GraphEngine) {
			_, err := eng.AddNode("person", map[string]any{"name": "b-keep"})
			Expect(err).NotTo(HaveOccurred())
		})

		Expect(runSyncCmd("--from", a, "--to", b)).To(Succeed())

		eng, err := LoadGraph(b)
		Expect(err).NotTo(HaveOccurred())
		Expect(findSyncNode(eng, "name", "a-x")).NotTo(BeNil())
		Expect(findSyncNode(eng, "name", "a-y")).NotTo(BeNil())
		Expect(findSyncNode(eng, "name", "b-keep")).NotTo(BeNil())
		x, y := findSyncNode(eng, "name", "a-x"), findSyncNode(eng, "name", "a-y")
		var found bool
		for _, e := range eng.Storage().GetEdgesFrom(x.ID) {
			if e.To == y.ID && e.Kind == "knows" {
				found = true
			}
		}
		Expect(found).To(BeTrue())

		// Re-sync after a source-side prop change: source wins.
		src, err := LoadGraph(a)
		Expect(err).NotTo(HaveOccurred())
		sx := findSyncNode(src, "name", "a-x")
		Expect(sx).NotTo(BeNil())
		Expect(src.Storage().UpdateNode(sx.ID, map[string]any{"name": "a-x", "city": "berlin"})).To(Succeed())
		Expect(SaveGraph(src, a)).To(Succeed())

		Expect(runSyncCmd("--from", "gob:"+a, "--to", "gob:"+b)).To(Succeed())
		eng, err = LoadGraph(b)
		Expect(err).NotTo(HaveOccurred())
		Expect(findSyncNode(eng, "city", "berlin")).NotTo(BeNil())
	})

	It("is additive by default and preserves --to-only nodes", func() {
		dir := GinkgoT().TempDir()
		a := filepath.Join(dir, "a.gob")
		b := filepath.Join(dir, "b.gob")
		mustWriteGob(a, func(eng *graph.GraphEngine) {
			_, err := eng.AddNode("person", map[string]any{"name": "a-only"})
			Expect(err).NotTo(HaveOccurred())
		})
		mustWriteGob(b, func(eng *graph.GraphEngine) {
			_, err := eng.AddNode("person", map[string]any{"name": "b-extra"})
			Expect(err).NotTo(HaveOccurred())
		})

		Expect(runSyncCmd("--from", "gob:"+a, "--to", "gob:"+b)).To(Succeed())

		eng, err := LoadGraph(b)
		Expect(err).NotTo(HaveOccurred())
		Expect(findSyncNode(eng, "name", "a-only")).NotTo(BeNil())
		Expect(findSyncNode(eng, "name", "b-extra")).NotTo(BeNil())
	})

	It("removes --to orphans with --prune", func() {
		dir := GinkgoT().TempDir()
		a := filepath.Join(dir, "a.gob")
		b := filepath.Join(dir, "b.gob")
		mustWriteGob(a, func(eng *graph.GraphEngine) {
			_, err := eng.AddNode("person", map[string]any{"name": "a-only"})
			Expect(err).NotTo(HaveOccurred())
		})
		mustWriteGob(b, func(eng *graph.GraphEngine) {
			e1, err := eng.AddNode("person", map[string]any{"name": "b-extra-1"})
			Expect(err).NotTo(HaveOccurred())
			e2, err := eng.AddNode("person", map[string]any{"name": "b-extra-2"})
			Expect(err).NotTo(HaveOccurred())
			Expect(eng.AddEdge(e1, e2, "links", nil)).To(Succeed())
		})

		Expect(runSyncCmd("--from", "gob:"+a, "--to", "gob:"+b, "--prune")).To(Succeed())

		eng, err := LoadGraph(b)
		Expect(err).NotTo(HaveOccurred())
		Expect(findSyncNode(eng, "name", "a-only")).NotTo(BeNil())
		Expect(findSyncNode(eng, "name", "b-extra-1")).To(BeNil())
		Expect(findSyncNode(eng, "name", "b-extra-2")).To(BeNil())
		Expect(eng.Storage().GetAllEdges()).To(BeEmpty())
	})

	It("errors honestly on cross-scheme pairs", func() {
		err := runSyncCmd("--from", "gob:a.gob", "--to", "sqlite:b.db")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("sync not implemented for gob -> sqlite yet"))
	})
})
