// Tier 2 DSL: Reach / ReachHas parse and execute end-to-end.
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

var _ = Describe("DSL Reach (Tier 2)", func() {
	var engine *graph.GraphEngine

	BeforeEach(func() {
		engine = graph.NewGraphEngine(inmem.New())
		a, _ := engine.AddNode("N", map[string]any{"name": "a"})
		b, _ := engine.AddNode("N", map[string]any{"name": "b"})
		c, _ := engine.AddNode("N", map[string]any{"name": "c"})
		d, _ := engine.AddNode("N", map[string]any{"name": "d"})
		_ = engine.AddEdge(a, b, "e", nil)
		_ = engine.AddEdge(b, c, "e", nil)
		_ = engine.AddEdge(c, d, "e", nil)
	})

	It("parses and executes Reach with explicit depth", func() {
		ast, err := dsl.NewParser("Find('N').Reach('e', 'out', 2)").Parse()
		Expect(err).NotTo(HaveOccurred())
		builder, err := ApplyAST(engine, ast)
		Expect(err).NotTo(HaveOccurred())

		result, err := builder.Exec(context.Background())
		Expect(err).NotTo(HaveOccurred())
		// 4 sources x reachable sets: a->{b,c}, b->{c,d}, c->{d}, d->{}
		Expect(result.Len()).To(Equal(2 + 2 + 1))
	})

	It("defaults direction to out and depth when omitted", func() {
		ast, err := dsl.NewParser("Find('N').Reach('e')").Parse()
		Expect(err).NotTo(HaveOccurred())
		builder, err := ApplyAST(engine, ast)
		Expect(err).NotTo(HaveOccurred())
		Expect(builder.ExportPlanForTest().Edges[0].MaxDepth).To(Equal(8))
	})

	It("execQuery runs a hit and a miss", func() {
		var hit bytes.Buffer
		err := execQuery(context.Background(), "Find('N').ReachHas('e', 'd', 'out', 3)", engine, &hit)
		Expect(err).NotTo(HaveOccurred())
		Expect(hit.String()).To(ContainSubstring("d(N)"))

		var miss bytes.Buffer
		err = execQuery(context.Background(), "Find('N').ReachHas('e', 'zzz', 'out', 3)", engine, &miss)
		Expect(err).NotTo(HaveOccurred())
		Expect(miss.String()).To(ContainSubstring("(no results)"))
	})

	It("rejects a bad direction and a bad depth type", func() {
		ast, err := dsl.NewParser("Find('N').Reach('e', 'sideways', 2)").Parse()
		Expect(err).NotTo(HaveOccurred())
		_, err = ApplyAST(engine, ast)
		Expect(err).To(HaveOccurred())

		ast, err = dsl.NewParser("Find('N').Reach('e', 'out', 'far')").Parse()
		Expect(err).NotTo(HaveOccurred())
		_, err = ApplyAST(engine, ast)
		Expect(err).To(HaveOccurred())
	})
})
