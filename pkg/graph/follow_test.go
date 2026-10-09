package graph_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/aprksy/knitknot/pkg/graph"
	"github.com/aprksy/knitknot/pkg/ports/query"
	"github.com/aprksy/knitknot/pkg/storage/inmem"
)

var _ = Describe("Builder.Follow (Tier 1 chains)", func() {
	var engine *graph.GraphEngine

	BeforeEach(func() {
		engine = graph.NewGraphEngine(inmem.New())
	})

	Context("fixed-length chain A -e1-> B -e2-> C", func() {
		It("resolves all three vars via chained Follow", func() {
			aID, _ := engine.AddNode("A", map[string]any{"name": "a"})
			bID, _ := engine.AddNode("B", map[string]any{"name": "b"})
			cID, _ := engine.AddNode("C", map[string]any{"name": "c"})
			_ = engine.AddEdge(aID, bID, "e1", nil)
			_ = engine.AddEdge(bID, cID, "e2", nil)

			plan := engine.Find("A").Follow("e1", query.Out).Follow("e2", query.Out).ExportPlanForTest()
			Expect(plan.Edges).To(HaveLen(2))
			Expect(plan.Edges[0].From).To(Equal("n"))
			Expect(plan.Edges[0].To).To(Equal("v0"))
			Expect(plan.Edges[1].From).To(Equal("v0"))
			Expect(plan.Edges[1].To).To(Equal("v1"))

			result, err := engine.Find("A").Follow("e1", query.Out).Follow("e2", query.Out).Exec(context.Background())
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Len()).To(Equal(1))
			row := result.Items()[0]
			Expect(row["n"].Props["name"]).To(Equal("a"))
			Expect(row["v0"].Props["name"]).To(Equal("b"))
			Expect(row["v1"].Props["name"]).To(Equal("c"))
		})
	})

	Context("mixed-direction CTI chain", func() {
		It("resolves actor <-attributed-to<- campaign ->uses-> malware", func() {
			actorID, _ := engine.AddNode("intrusion-set", map[string]any{"name": "actor"})
			campID, _ := engine.AddNode("campaign", map[string]any{"name": "camp"})
			malID, _ := engine.AddNode("malware", map[string]any{"name": "mal"})
			_ = engine.AddEdge(campID, actorID, "attributed-to", nil)
			_ = engine.AddEdge(campID, malID, "uses", nil)

			result, err := engine.Find("intrusion-set").
				Follow("attributed-to", query.In).
				Follow("uses", query.Out).
				Exec(context.Background())
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Len()).To(Equal(1))
			row := result.Items()[0]
			Expect(row["n"].Props["name"]).To(Equal("actor"))
			Expect(row["v0"].Props["name"]).To(Equal("camp"))
			Expect(row["v1"].Props["name"]).To(Equal("mal"))
		})
	})

	Context("directions", func() {
		It("In traverses incoming edges", func() {
			aID, _ := engine.AddNode("A", map[string]any{"name": "a"})
			bID, _ := engine.AddNode("B", map[string]any{"name": "b"})
			_ = engine.AddEdge(aID, bID, "e1", nil)

			result, err := engine.Find("B").Follow("e1", query.In).Exec(context.Background())
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Len()).To(Equal(1))
			Expect(result.Items()[0]["v0"].Props["name"]).To(Equal("a"))
		})

		It("Out ignores incoming edges", func() {
			aID, _ := engine.AddNode("A", map[string]any{"name": "a"})
			bID, _ := engine.AddNode("B", map[string]any{"name": "b"})
			_ = engine.AddEdge(aID, bID, "e1", nil)

			result, err := engine.Find("B").Follow("e1", query.Out).Exec(context.Background())
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Empty()).To(BeTrue())
		})

		It("Both unions incoming and outgoing", func() {
			aID, _ := engine.AddNode("N", map[string]any{"name": "a"})
			mID, _ := engine.AddNode("N", map[string]any{"name": "m"})
			zID, _ := engine.AddNode("N", map[string]any{"name": "z"})
			_ = engine.AddEdge(aID, mID, "e", nil)
			_ = engine.AddEdge(mID, zID, "e", nil)

			result, err := engine.Find("N").Where("n.name", "=", "m").Follow("e", query.Both).Exec(context.Background())
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Len()).To(Equal(2))
		})

		It("returns empty when nothing matches", func() {
			_, _ = engine.AddNode("A", map[string]any{"name": "a"})
			result, err := engine.Find("A").Follow("missing", query.Out).Exec(context.Background())
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Empty()).To(BeTrue())

			result, err = engine.Find("A").Follow("missing", query.In).Exec(context.Background())
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Empty()).To(BeTrue())
		})

		It("dedups targets within a row", func() {
			aID, _ := engine.AddNode("N", map[string]any{"name": "a"})
			bID, _ := engine.AddNode("N", map[string]any{"name": "b"})
			_ = engine.AddEdge(aID, bID, "e", nil)
			_ = engine.AddEdge(aID, bID, "e", nil)

			result, err := engine.Find("N").Where("n.name", "=", "a").Follow("e", query.Both).Exec(context.Background())
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Len()).To(Equal(1))
		})
	})

	Context("FollowHas", func() {
		It("chains with a value filter", func() {
			aID, _ := engine.AddNode("A", map[string]any{"name": "a"})
			bID, _ := engine.AddNode("B", map[string]any{"name": "want"})
			cID, _ := engine.AddNode("B", map[string]any{"name": "other"})
			_ = engine.AddEdge(aID, bID, "e1", nil)
			_ = engine.AddEdge(aID, cID, "e1", nil)

			result, err := engine.Find("A").FollowHas("e1", "want", query.Out).Exec(context.Background())
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Len()).To(Equal(1))
			Expect(result.Items()[0]["v0"].Props["name"]).To(Equal("want"))
		})

		It("chains the next Follow from the filtered hop", func() {
			aID, _ := engine.AddNode("A", map[string]any{"name": "a"})
			bID, _ := engine.AddNode("B", map[string]any{"name": "want"})
			cID, _ := engine.AddNode("C", map[string]any{"name": "c"})
			_ = engine.AddEdge(aID, bID, "e1", nil)
			_ = engine.AddEdge(bID, cID, "e2", nil)

			plan := engine.Find("A").FollowHas("e1", "want", query.Out).Follow("e2", query.Out).ExportPlanForTest()
			Expect(plan.Edges[1].From).To(Equal("v0"))

			result, err := engine.Find("A").FollowHas("e1", "want", query.Out).Follow("e2", query.Out).Exec(context.Background())
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Len()).To(Equal(1))
			Expect(result.Items()[0]["v1"].Props["name"]).To(Equal("c"))
		})
	})

	Context("backward compatibility", func() {
		It("Has still fans out from n", func() {
			aID, _ := engine.AddNode("A", map[string]any{"name": "a"})
			bID, _ := engine.AddNode("B", map[string]any{"name": "x"})
			cID, _ := engine.AddNode("C", map[string]any{"name": "y"})
			_ = engine.AddEdge(aID, bID, "e1", nil)
			_ = engine.AddEdge(aID, cID, "e2", nil)

			plan := engine.Find("A").Has("e1", "x").Has("e2", "y").ExportPlanForTest()
			for _, e := range plan.Edges {
				Expect(e.From).To(Equal("n"))
			}

			result, err := engine.Find("A").Has("e1", "x").Has("e2", "y").Exec(context.Background())
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Len()).To(Equal(1))
		})

		It("Follow after Has chains from the Has target", func() {
			aID, _ := engine.AddNode("A", map[string]any{"name": "a"})
			bID, _ := engine.AddNode("B", map[string]any{"name": "b"})
			cID, _ := engine.AddNode("C", map[string]any{"name": "c"})
			_ = engine.AddEdge(aID, bID, "e1", nil)
			_ = engine.AddEdge(bID, cID, "e2", nil)

			plan := engine.Find("A").Has("e1", "b").Follow("e2", query.Out).ExportPlanForTest()
			Expect(plan.Edges[1].From).To(Equal("v0"))

			result, err := engine.Find("A").Has("e1", "b").Follow("e2", query.Out).Exec(context.Background())
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Len()).To(Equal(1))
			Expect(result.Items()[0]["v1"].Props["name"]).To(Equal("c"))
		})

		It("zero-value Direction defaults to Out", func() {
			aID, _ := engine.AddNode("A", map[string]any{"name": "a"})
			bID, _ := engine.AddNode("B", map[string]any{"name": "b"})
			_ = engine.AddEdge(aID, bID, "e1", nil)

			result, err := engine.Find("A").Follow("e1", query.Direction(0)).Exec(context.Background())
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Len()).To(Equal(1))
		})
	})
})
