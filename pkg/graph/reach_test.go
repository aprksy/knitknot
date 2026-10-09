package graph_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/aprksy/knitknot/pkg/graph"
	"github.com/aprksy/knitknot/pkg/ports/query"
	"github.com/aprksy/knitknot/pkg/storage/inmem"
)

var _ = Describe("Builder.Reach (Tier 2)", func() {
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

	Context("depth handling", func() {
		It("defaults to DefaultReachDepth", func() {
			plan := engine.Find("N").Reach("e", query.Out).ExportPlanForTest()
			Expect(plan.Edges).To(HaveLen(1))
			Expect(plan.Edges[0].MaxDepth).To(Equal(query.DefaultReachDepth))
			Expect(plan.Edges[0].MinDepth).To(Equal(1))
		})

		It("accepts an explicit depth", func() {
			plan := engine.Find("N").Reach("e", query.Out, 3).ExportPlanForTest()
			Expect(plan.Edges[0].MaxDepth).To(Equal(3))
		})

		It("clamps above MaxReachDepth and below 1", func() {
			plan := engine.Find("N").Reach("e", query.Out, 99).ExportPlanForTest()
			Expect(plan.Edges[0].MaxDepth).To(Equal(query.MaxReachDepth))
			plan = engine.Find("N").Reach("e", query.Out, 0).ExportPlanForTest()
			Expect(plan.Edges[0].MaxDepth).To(Equal(1))
			plan = engine.Find("N").Reach("e", query.Out, -5).ExportPlanForTest()
			Expect(plan.Edges[0].MaxDepth).To(Equal(1))
		})

		It("chains from the last var and updates it", func() {
			plan := engine.Find("N").Follow("e", query.Out).Reach("e", query.Out, 2).ExportPlanForTest()
			Expect(plan.Edges[1].From).To(Equal("v0"))
			Expect(plan.Edges[1].To).To(Equal("v1"))

			plan = engine.Find("N").Reach("e", query.Out, 2).Reach("e", query.Out, 2).ExportPlanForTest()
			Expect(plan.Edges[1].From).To(Equal("v0"))
		})
	})

	Context("execution", func() {
		It("resolves the bounded node set", func() {
			result, err := engine.Find("N").Where("n.name", "=", "a").Reach("e", query.Out, 2).Exec(context.Background())
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Len()).To(Equal(2))

			result, err = engine.Find("N").Where("n.name", "=", "a").Reach("e", query.Out, 3).Exec(context.Background())
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Len()).To(Equal(3))
		})

		It("ReachHas filters reached nodes by value", func() {
			result, err := engine.Find("N").Where("n.name", "=", "a").
				ReachHas("e", "c", query.Out, 3).Exec(context.Background())
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Len()).To(Equal(1))
			Expect(result.Items()[0]["v0"].Props["name"]).To(Equal("c"))

			miss, err := engine.Find("N").Where("n.name", "=", "a").
				ReachHas("e", "nope", query.Out, 3).Exec(context.Background())
			Expect(err).NotTo(HaveOccurred())
			Expect(miss.Empty()).To(BeTrue())
		})
	})

	Context("regression", func() {
		It("Has fan-out and Follow chains are unchanged", func() {
			a, _ := engine.AddNode("A", map[string]any{"name": "a"})
			b, _ := engine.AddNode("B", map[string]any{"name": "b"})
			_ = engine.AddEdge(a, b, "e", nil)

			has, err := engine.Find("A").Has("e", "b").Exec(context.Background())
			Expect(err).NotTo(HaveOccurred())
			Expect(has.Len()).To(Equal(1))

			follow, err := engine.Find("A").Follow("e", query.Out).Exec(context.Background())
			Expect(err).NotTo(HaveOccurred())
			Expect(follow.Len()).To(Equal(1))
			Expect(follow.Items()[0]["v0"].Props["name"]).To(Equal("b"))
		})
	})
})
