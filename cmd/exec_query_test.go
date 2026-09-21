// H3 + M1 + M2 regression: execQuery numeric coercion, un-prefixed filters, .In() scoping.
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

var _ = Describe("execQuery / ApplyAST", func() {
	var (
		engine *graph.GraphEngine
		st     *inmem.Storage
		saved  struct {
			subgraph string
			file     string
		}
	)

	seedQueryEngine := func() (*graph.GraphEngine, *inmem.Storage) {
		s := inmem.New()
		e := graph.NewGraphEngine(s)
		_, _ = e.AddNode("customer", map[string]any{"name": "Alice", "age": "51", "city": "Dallas"})
		_, _ = e.AddNode("customer", map[string]any{"name": "Bob", "age": "40", "city": "Austin"})
		_, _ = e.AddNode("customer", map[string]any{"name": "Carol", "age": "33", "city": "Austin"})
		return e, s
	}

	BeforeEach(func() {
		saved = globalFlags
		engine, st = seedQueryEngine()
		_ = st
	})

	AfterEach(func() {
		globalFlags = saved
	})

	It("M1: coerces stored string age against numeric filter", func() {
		var out bytes.Buffer
		err := execQuery(context.Background(), "Find('customer').Where('n.age', '=', 51)", engine, &out)
		Expect(err).NotTo(HaveOccurred())
		Expect(out.String()).To(ContainSubstring("Alice"))
		Expect(out.String()).To(ContainSubstring("1 result"))
	})

	It("M2: un-prefixed filter applies to first var only", func() {
		var out bytes.Buffer
		err := execQuery(context.Background(), "Find('customer').Where('city', '=', 'Dallas')", engine, &out)
		Expect(err).NotTo(HaveOccurred())
		Expect(out.String()).To(ContainSubstring("Alice"))
		Expect(out.String()).NotTo(ContainSubstring("Bob"))
		Expect(out.String()).NotTo(ContainSubstring("Carol"))
		Expect(out.String()).To(ContainSubstring("1 result"))
	})

	It("ApplyAST rejects Find(5) without panicking", func() {
		ast := &dsl.Query{Methods: []*dsl.MethodCall{
			{Name: &dsl.Identifier{Value: "Find"}, Arguments: []dsl.Expression{&dsl.NumberLiteral{Value: 5}}},
		}}
		var builder *graph.Builder
		Expect(func() {
			var err error
			builder, err = ApplyAST(engine, ast)
			Expect(err).To(HaveOccurred())
		}).NotTo(Panic())
		Expect(builder).To(BeNil())
	})

	It("H3: ApplyAST wires .In() onto the plan", func() {
		p := dsl.NewParser("Find('X').In('sub')")
		ast, err := p.Parse()
		Expect(err).NotTo(HaveOccurred())
		builder, err := ApplyAST(engine, ast)
		Expect(err).NotTo(HaveOccurred())
		Expect(builder.ExportPlanForTest().Subgraph).To(Equal("sub"))
	})

	It("H3: .In('nope') scopes out nodes living in 'org'", func() {
		for _, n := range st.GetAllNodes() {
			st.AddToSubgraph(n, "org", "")
		}
		var out bytes.Buffer
		err := execQuery(context.Background(), "Find('customer').In('nope')", engine, &out)
		Expect(err).NotTo(HaveOccurred())
		Expect(out.String()).To(ContainSubstring("(no results)"))

		var hit bytes.Buffer
		err = execQuery(context.Background(), "Find('customer').In('org')", engine, &hit)
		Expect(err).NotTo(HaveOccurred())
		Expect(hit.String()).To(ContainSubstring("3 result"))
	})
})
