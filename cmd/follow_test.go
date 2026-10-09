// Tier 1 DSL: Follow / FollowHas parse and execute end-to-end.
package cmd

import (
	"bytes"
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/aprksy/knitknot/pkg/dsl"
	"github.com/aprksy/knitknot/pkg/graph"
	"github.com/aprksy/knitknot/pkg/storage/inmem"
)

var _ = Describe("DSL Follow (Tier 1)", func() {
	var engine *graph.GraphEngine

	BeforeEach(func() {
		engine = graph.NewGraphEngine(inmem.New())
		aID, _ := engine.AddNode("A", map[string]any{"name": "a"})
		bID, _ := engine.AddNode("B", map[string]any{"name": "b"})
		cID, _ := engine.AddNode("C", map[string]any{"name": "c"})
		_ = engine.AddEdge(aID, bID, "e1", nil)
		_ = engine.AddEdge(bID, cID, "e2", nil)
	})

	It("parses and executes a two-hop chain", func() {
		ast, err := dsl.NewParser("Find('A').Follow('e1').Follow('e2')").Parse()
		Expect(err).NotTo(HaveOccurred())
		builder, err := ApplyAST(engine, ast)
		Expect(err).NotTo(HaveOccurred())

		result, err := builder.Exec(context.Background())
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Len()).To(Equal(1))
		row := result.Items()[0]
		Expect(row["n"].Props["name"]).To(Equal("a"))
		Expect(row["v0"].Props["name"]).To(Equal("b"))
		Expect(row["v1"].Props["name"]).To(Equal("c"))
	})

	It("execQuery runs the chain end-to-end", func() {
		var out bytes.Buffer
		err := execQuery(context.Background(), "Find('A').Follow('e1').Follow('e2')", engine, &out)
		Expect(err).NotTo(HaveOccurred())
		Expect(out.String()).To(ContainSubstring("1 result"))
	})

	It("honors the 'in' direction", func() {
		var out bytes.Buffer
		err := execQuery(context.Background(), "Find('C').Follow('e2', 'in').Follow('e1', 'in')", engine, &out)
		Expect(err).NotTo(HaveOccurred())
		Expect(out.String()).To(ContainSubstring("1 result"))
		Expect(out.String()).To(ContainSubstring("a"))
	})

	It("FollowHas filters the hop target", func() {
		var out bytes.Buffer
		err := execQuery(context.Background(), "Find('A').FollowHas('e1', 'b').Follow('e2')", engine, &out)
		Expect(err).NotTo(HaveOccurred())
		Expect(out.String()).To(ContainSubstring("1 result"))

		var miss bytes.Buffer
		err = execQuery(context.Background(), "Find('A').FollowHas('e1', 'nope').Follow('e2')", engine, &miss)
		Expect(err).NotTo(HaveOccurred())
		Expect(miss.String()).To(ContainSubstring("(no results)"))
	})

	It("rejects a bad direction", func() {
		ast, err := dsl.NewParser("Find('A').Follow('e1', 'sideways')").Parse()
		Expect(err).NotTo(HaveOccurred())
		_, err = ApplyAST(engine, ast)
		Expect(err).To(HaveOccurred())
	})
})
