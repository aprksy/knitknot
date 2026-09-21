// H2 + L8 regression: execUpdateEdge parses trailing props; missing edge reports "edge not found".
package cmd

import (
	"bytes"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/aprksy/knitknot/pkg/graph"
	"github.com/aprksy/knitknot/pkg/storage/inmem"
)

var _ = Describe("execUpdateNode / execUpdateEdge", func() {
	var (
		engine *graph.GraphEngine
		aID    string
		bID    string
		saved  struct {
			subgraph string
			file     string
		}
	)

	BeforeEach(func() {
		saved = globalFlags
		engine = graph.NewGraphEngine(inmem.New())
		var err error
		aID, err = engine.AddNode("A", map[string]any{"name": "a"})
		Expect(err).NotTo(HaveOccurred())
		bID, err = engine.AddNode("B", map[string]any{"name": "b"})
		Expect(err).NotTo(HaveOccurred())
		Expect(engine.AddEdge(aID, bID, "likes", map[string]any{})).To(Succeed())
	})

	AfterEach(func() {
		globalFlags = saved
	})

	// NOTE: the BeforeEach edge seeds empty (non-nil) props for the plain
	// H2 cases; the nil-props edge case is covered by its own spec below
	// (the nil-map write used to panic at repl_update.go:110).

	It("H2: applies a single trailing prop", func() {
		var out bytes.Buffer
		err := execUpdateEdge(engine, fmt.Sprintf("%s --likes--> %s priority=high", aID, bID), &out)
		Expect(err).NotTo(HaveOccurred())
		Expect(out.String()).To(ContainSubstring("Updated edge"))
		edge, ok := engine.GetEdge(fmt.Sprintf("%s->%s@likes", aID, bID))
		Expect(ok).To(BeTrue())
		Expect(edge.Props["priority"]).To(Equal("high"))
	})

	It("nil-props edge update does not panic (repl_update.go:110 guard)", func() {
		cID, err := engine.AddNode("C", nil)
		Expect(err).NotTo(HaveOccurred())
		// An edge added with nil props has a nil Props map; writing to it
		// used to panic before the nil-map guard landed.
		Expect(engine.AddEdge(aID, cID, "knows", nil)).To(Succeed())
		var out bytes.Buffer
		err = execUpdateEdge(engine, fmt.Sprintf("%s --knows--> %s weight=5", aID, cID), &out)
		Expect(err).NotTo(HaveOccurred())
		edge, ok := engine.GetEdge(fmt.Sprintf("%s->%s@knows", aID, cID))
		Expect(ok).To(BeTrue())
		Expect(edge.Props["weight"]).To(Equal(5))
	})

	It("H2: applies multiple trailing props", func() {
		var out bytes.Buffer
		err := execUpdateEdge(engine, fmt.Sprintf("%s --likes--> %s priority=high important=true", aID, bID), &out)
		Expect(err).NotTo(HaveOccurred())
		edge, ok := engine.GetEdge(fmt.Sprintf("%s->%s@likes", aID, bID))
		Expect(ok).To(BeTrue())
		Expect(edge.Props["priority"]).To(Equal("high"))
		Expect(edge.Props["important"]).To(Equal("true"))
	})

	It("L8: missing edge reports 'edge not found'", func() {
		var out bytes.Buffer
		// NOTE: a trailing prop is required to reach the edge-exists check;
		// without props the command reports "no properties to update".
		err := execUpdateEdge(engine, fmt.Sprintf("%s --likes--> X priority=high", aID), &out)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("edge not found"))
	})

	It("execUpdateNode with bad id returns error", func() {
		var out bytes.Buffer
		err := execUpdateNode(engine, "nope k=v", &out)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("not found"))
	})
})
