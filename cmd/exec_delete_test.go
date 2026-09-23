// M3 regression: deleting a node removes its incident edges (no dangling edges).
package cmd

import (
	"bytes"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/aprksy/knitknot/pkg/graph"
	"github.com/aprksy/knitknot/pkg/storage/inmem"
)

var _ = Describe("execDelete / execDeleteEdge", func() {
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

	It("M3: deleting a node drops incident edges", func() {
		aID, err := engine.AddNode("A", map[string]any{"name": "a"})
		Expect(err).NotTo(HaveOccurred())
		bID, err := engine.AddNode("B", map[string]any{"name": "b"})
		Expect(err).NotTo(HaveOccurred())
		Expect(engine.AddEdge(aID, bID, "rel", nil)).To(Succeed())

		var out bytes.Buffer
		Expect(execDelete(engine, "NODE "+aID, &out)).To(Succeed())
		Expect(out.String()).To(ContainSubstring("Deleted node"))
		st := engine.Storage().(*inmem.Storage)
		Expect(st.GetAllEdges()).To(BeEmpty())
		// version: tombstone — node stays findable with DeletedAt set.
		tomb, ok := engine.GetNode(aID)
		Expect(ok).To(BeTrue())
		Expect(tomb.DeletedAt).NotTo(BeNil())
	})

	It("deletes an edge via EDGE spec", func() {
		aID, _ := engine.AddNode("A", map[string]any{"name": "a"})
		bID, _ := engine.AddNode("B", map[string]any{"name": "b"})
		Expect(engine.AddEdge(aID, bID, "rel", nil)).To(Succeed())

		var out bytes.Buffer
		Expect(execDelete(engine, fmt.Sprintf("EDGE %s --rel--> %s", aID, bID), &out)).To(Succeed())
		st := engine.Storage().(*inmem.Storage)
		Expect(st.GetAllEdges()).To(BeEmpty())
	})

	It("rejects nonsense input with usage error", func() {
		var out bytes.Buffer
		err := execDelete(engine, "nonsense", &out)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("usage"))
	})

	It("rejects bare NODE with an error", func() {
		var out bytes.Buffer
		err := execDelete(engine, "NODE", &out)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(SatisfyAny(ContainSubstring("missing"), ContainSubstring("usage")))
	})
})
