// L7 regression: execSave never nil-derefs Stat; plus Save/Load round-trip (H6 version header).
package cmd

import (
	"bytes"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/aprksy/knitknot/pkg/graph"
	"github.com/aprksy/knitknot/pkg/storage/inmem"
)

var _ = Describe("execSave / execLoad", func() {
	var (
		engine *graph.GraphEngine
		saved  struct {
			subgraph string
			file     string
		}
	)

	BeforeEach(func() {
		saved = globalFlags
		engine = graph.NewGraphEngine(inmem.New())
	})

	AfterEach(func() {
		globalFlags = saved
	})

	tmpPath := func() string {
		dir := GinkgoT().TempDir()
		f, err := os.CreateTemp(dir, "*.gob")
		Expect(err).NotTo(HaveOccurred())
		path := f.Name()
		Expect(f.Close()).To(Succeed())
		Expect(os.Remove(path)).To(Succeed())
		return path
	}

	It("L7: empty filename is 'missing filename'", func() {
		var out bytes.Buffer
		err := execSave(engine, "", &out)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("missing filename"))
	})

	It("L7: re-saving after external remove does not panic", func() {
		path := tmpPath()
		var out bytes.Buffer
		Expect(execSave(engine, path, &out)).To(Succeed())
		Expect(os.Remove(path)).To(Succeed())
		var again bytes.Buffer
		Expect(func() {
			Expect(execSave(engine, path, &again)).To(Succeed())
		}).NotTo(Panic())
		Expect(again.String()).To(ContainSubstring("Saved"))
	})

	It("round-trips nodes and edges through Load", func() {
		aID, err := engine.AddNode("Person", map[string]any{"name": "Alice"})
		Expect(err).NotTo(HaveOccurred())
		bID, err := engine.AddNode("Person", map[string]any{"name": "Bob"})
		Expect(err).NotTo(HaveOccurred())
		Expect(engine.AddEdge(aID, bID, "knows", nil)).To(Succeed())

		path := filepath.Join(GinkgoT().TempDir(), "g.gob")
		var out bytes.Buffer
		Expect(execSave(engine, path, &out)).To(Succeed())
		Expect(out.String()).To(ContainSubstring("Saved"))
		info, err := os.Stat(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(info.Size()).To(BeNumerically(">", 0))

		fresh := graph.NewGraphEngine(inmem.New())
		var lout bytes.Buffer
		Expect(execLoad(fresh, path, &lout)).To(Succeed())
		st := fresh.Storage().(*inmem.Storage)
		Expect(st.GetAllNodes()).To(HaveLen(2))
		Expect(st.GetAllEdges()).To(HaveLen(1))
	})

	It("execLoad with empty filename is 'missing filename'", func() {
		var out bytes.Buffer
		err := execLoad(engine, "", &out)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("missing filename"))
	})

	It("execLoad of missing path is 'file not found'", func() {
		var out bytes.Buffer
		err := execLoad(engine, filepath.Join(GinkgoT().TempDir(), "nonexistent-path-xxx"), &out)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("file not found"))
	})
})
