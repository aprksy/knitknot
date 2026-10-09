package query_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/aprksy/knitknot/pkg/graph"
	q "github.com/aprksy/knitknot/pkg/ports/query"
	"github.com/aprksy/knitknot/pkg/query"
	"github.com/aprksy/knitknot/pkg/storage/inmem"
)

var _ = Describe("DefaultQueryEngine reachability (Tier 2)", func() {
	var (
		storage *inmem.Storage
		engine  *graph.GraphEngine
		qe      *query.DefaultQueryEngine
	)

	BeforeEach(func() {
		storage = inmem.New()
		engine = graph.NewGraphEngine(storage)
		qe = query.NewDefaultQueryEngine()
	})

	// a -e-> b -e-> c -e-> d
	chain4 := func() {
		a, _ := engine.AddNode("N", map[string]any{"name": "a"})
		b, _ := engine.AddNode("N", map[string]any{"name": "b"})
		c, _ := engine.AddNode("N", map[string]any{"name": "c"})
		d, _ := engine.AddNode("N", map[string]any{"name": "d"})
		_ = engine.AddEdge(a, b, "e", nil)
		_ = engine.AddEdge(b, c, "e", nil)
		_ = engine.AddEdge(c, d, "e", nil)
	}

	reachPlan := func(edge *q.PatternEdge) *q.QueryPlan {
		return &q.QueryPlan{
			Nodes: []*q.PatternNode{
				{Var: "n", Label: "N"},
				{Var: "v0", Label: "N"},
			},
			Edges: []*q.PatternEdge{edge},
			Filters: []q.Filter{
				{Field: "n.name", Op: "=", Value: "a"},
			},
		}
	}

	names := func(rs *query.ResultSet) []string {
		var out []string
		for _, row := range rs.Items() {
			out = append(out, row["v0"].Props["name"].(string))
		}
		return out
	}

	It("reaches 1..N and stops at the bound", func() {
		chain4()
		deep, err := qe.Execute(context.Background(), storage,
			reachPlan(&q.PatternEdge{From: "n", To: "v0", Kind: "e", MaxDepth: 8}))
		Expect(err).NotTo(HaveOccurred())
		Expect(names(deep.(*query.ResultSet))).To(ConsistOf("b", "c", "d"))

		bounded, err := qe.Execute(context.Background(), storage,
			reachPlan(&q.PatternEdge{From: "n", To: "v0", Kind: "e", MaxDepth: 2}))
		Expect(err).NotTo(HaveOccurred())
		Expect(names(bounded.(*query.ResultSet))).To(ConsistOf("b", "c"))
	})

	It("honors MinDepth", func() {
		chain4()
		result, err := qe.Execute(context.Background(), storage,
			reachPlan(&q.PatternEdge{From: "n", To: "v0", Kind: "e", MinDepth: 2, MaxDepth: 3}))
		Expect(err).NotTo(HaveOccurred())
		Expect(names(result.(*query.ResultSet))).To(ConsistOf("c", "d"))
	})

	It("MaxDepth <= 1 keeps single-hop behavior", func() {
		chain4()
		zero, err := qe.Execute(context.Background(), storage,
			reachPlan(&q.PatternEdge{From: "n", To: "v0", Kind: "e"}))
		Expect(err).NotTo(HaveOccurred())
		one, err := qe.Execute(context.Background(), storage,
			reachPlan(&q.PatternEdge{From: "n", To: "v0", Kind: "e", MaxDepth: 1}))
		Expect(err).NotTo(HaveOccurred())
		Expect(names(zero.(*query.ResultSet))).To(ConsistOf("b"))
		Expect(names(one.(*query.ResultSet))).To(ConsistOf("b"))
	})

	It("terminates on cycles and never emits the source", func() {
		a, _ := engine.AddNode("N", map[string]any{"name": "a"})
		b, _ := engine.AddNode("N", map[string]any{"name": "b"})
		c, _ := engine.AddNode("N", map[string]any{"name": "c"})
		_ = engine.AddEdge(a, b, "e", nil)
		_ = engine.AddEdge(b, c, "e", nil)
		_ = engine.AddEdge(c, a, "e", nil)

		result, err := qe.Execute(context.Background(), storage,
			reachPlan(&q.PatternEdge{From: "n", To: "v0", Kind: "e", MaxDepth: 8}))
		Expect(err).NotTo(HaveOccurred())
		Expect(names(result.(*query.ResultSet))).To(ConsistOf("b", "c"))
	})

	It("dedups nodes reached via multiple paths", func() {
		a, _ := engine.AddNode("N", map[string]any{"name": "a"})
		b, _ := engine.AddNode("N", map[string]any{"name": "b"})
		c, _ := engine.AddNode("N", map[string]any{"name": "c"})
		d, _ := engine.AddNode("N", map[string]any{"name": "d"})
		_ = engine.AddEdge(a, b, "e", nil)
		_ = engine.AddEdge(a, c, "e", nil)
		_ = engine.AddEdge(b, d, "e", nil)
		_ = engine.AddEdge(c, d, "e", nil)

		result, err := qe.Execute(context.Background(), storage,
			reachPlan(&q.PatternEdge{From: "n", To: "v0", Kind: "e", MaxDepth: 3}))
		Expect(err).NotTo(HaveOccurred())
		Expect(names(result.(*query.ResultSet))).To(ConsistOf("b", "c", "d"))
	})

	It("traverses In and Both", func() {
		chain4()

		plan := reachPlan(&q.PatternEdge{From: "n", To: "v0", Kind: "e", Direction: q.In, MaxDepth: 2})
		plan.Filters = []q.Filter{{Field: "n.name", Op: "=", Value: "c"}}
		result, err := qe.Execute(context.Background(), storage, plan)
		Expect(err).NotTo(HaveOccurred())
		Expect(names(result.(*query.ResultSet))).To(ConsistOf("b", "a"))

		plan = reachPlan(&q.PatternEdge{From: "n", To: "v0", Kind: "e", Direction: q.Both, MaxDepth: 1})
		plan.Filters = []q.Filter{{Field: "n.name", Op: "=", Value: "b"}}
		result, err = qe.Execute(context.Background(), storage, plan)
		Expect(err).NotTo(HaveOccurred())
		Expect(names(result.(*query.ResultSet))).To(ConsistOf("a", "c"))
	})

	It("filters by kind, with empty kind matching any", func() {
		a, _ := engine.AddNode("N", map[string]any{"name": "a"})
		b, _ := engine.AddNode("N", map[string]any{"name": "b"})
		c, _ := engine.AddNode("N", map[string]any{"name": "c"})
		_ = engine.AddEdge(a, b, "e1", nil)
		_ = engine.AddEdge(b, c, "e2", nil)

		result, err := qe.Execute(context.Background(), storage,
			reachPlan(&q.PatternEdge{From: "n", To: "v0", Kind: "e1", MaxDepth: 3}))
		Expect(err).NotTo(HaveOccurred())
		Expect(names(result.(*query.ResultSet))).To(ConsistOf("b"))

		result, err = qe.Execute(context.Background(), storage,
			reachPlan(&q.PatternEdge{From: "n", To: "v0", MaxDepth: 3}))
		Expect(err).NotTo(HaveOccurred())
		Expect(names(result.(*query.ResultSet))).To(ConsistOf("b", "c"))
	})

	It("traverses through label-mismatched nodes but only emits matches", func() {
		a, _ := engine.AddNode("A", map[string]any{"name": "a"})
		b, _ := engine.AddNode("B", map[string]any{"name": "b"})
		c, _ := engine.AddNode("C", map[string]any{"name": "c"})
		_ = engine.AddEdge(a, b, "e", nil)
		_ = engine.AddEdge(b, c, "e", nil)

		plan := &q.QueryPlan{
			Nodes: []*q.PatternNode{
				{Var: "n", Label: "A"},
				{Var: "v0", Label: "C"},
			},
			Edges:   []*q.PatternEdge{{From: "n", To: "v0", Kind: "e", MaxDepth: 3}},
			Filters: []q.Filter{{Field: "n.name", Op: "=", Value: "a"}},
		}
		result, err := qe.Execute(context.Background(), storage, plan)
		Expect(err).NotTo(HaveOccurred())
		Expect(names(result.(*query.ResultSet))).To(ConsistOf("c"))
	})

	It("restricts BFS traversal and emission to the subgraph", func() {
		a, _ := engine.AddNode("N", map[string]any{"name": "a"})
		b, _ := engine.AddNode("N", map[string]any{"name": "b"})
		c, _ := engine.AddNode("N", map[string]any{"name": "c"})
		x, _ := engine.AddNode("N", map[string]any{"name": "x"})
		_ = engine.AddEdge(a, b, "e", nil)
		_ = engine.AddEdge(b, c, "e", nil)
		_ = engine.AddEdge(b, x, "e", nil)
		for _, id := range []string{a, b, c} {
			n, _ := storage.GetNode(id)
			storage.AddToSubgraph(n, "sub", "")
		}

		plan := reachPlan(&q.PatternEdge{From: "n", To: "v0", Kind: "e", MaxDepth: 3})
		plan.Subgraph = "sub"
		result, err := qe.Execute(context.Background(), storage, plan)
		Expect(err).NotTo(HaveOccurred())
		Expect(names(result.(*query.ResultSet))).To(ConsistOf("b", "c"))

		plan.Subgraph = ""
		result, err = qe.Execute(context.Background(), storage, plan)
		Expect(err).NotTo(HaveOccurred())
		Expect(names(result.(*query.ResultSet))).To(ConsistOf("b", "c", "x"))
	})

	It("does not mutate the graph", func() {
		chain4()
		beforeNodes := len(storage.GetAllNodes())
		beforeEdges := len(storage.GetEdgesByKind("e"))

		_, err := qe.Execute(context.Background(), storage,
			reachPlan(&q.PatternEdge{From: "n", To: "v0", Kind: "e", MaxDepth: 8}))
		Expect(err).NotTo(HaveOccurred())
		Expect(len(storage.GetAllNodes())).To(Equal(beforeNodes))
		Expect(len(storage.GetEdgesByKind("e"))).To(Equal(beforeEdges))
	})
})
