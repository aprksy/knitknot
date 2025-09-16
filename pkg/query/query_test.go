package query_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/aprksy/knitknot/pkg/graph"
	q "github.com/aprksy/knitknot/pkg/ports/query"
	"github.com/aprksy/knitknot/pkg/ports/types"
	"github.com/aprksy/knitknot/pkg/query"
	"github.com/aprksy/knitknot/pkg/storage/inmem"
)

var _ = Describe("DefaultQueryEngine", func() {
	var (
		storage *inmem.Storage
		engine  *graph.GraphEngine
		qe      *query.DefaultQueryEngine
	)

	beforeEach := func() {
		storage = inmem.New()
		engine = graph.NewGraphEngine(storage)
		qe = query.NewDefaultQueryEngine()
	}

	describeExecution := func(name string, filter q.Filter, setup func(), verify func(*query.ResultSet)) {
		It(name, func() {
			beforeEach()
			setup()
			plan := &q.QueryPlan{
				Nodes: []*q.PatternNode{
					{Var: "n", Label: "User"},
				},
				Filters: []q.Filter{filter},
			}
			result, err := qe.Execute(context.Background(), storage, plan)
			Expect(err).NotTo(HaveOccurred())
			verify(result.(*query.ResultSet))
		})
	}

	Context("when executing simple node queries ==", func() {
		describeExecution("should find node by label and property",
			q.Filter{Field: "n.name", Op: "=", Value: "Alice"},
			func() {
				_, _ = engine.AddNode("User", map[string]any{"name": "Alice"})
				_, _ = engine.AddNode("User", map[string]any{"name": "Bob"})
			},
			func(result *query.ResultSet) {
				Expect(result.Len()).To(Equal(1))
				row := result.Items()[0]
				node, ok := row["n"]
				Expect(ok).To(BeTrue())
				Expect(node.Props["name"]).To(Equal("Alice"))
			},
		)
	})

	Context("when executing simple node queries with !=", func() {
		describeExecution("should find node by label and property",
			q.Filter{Field: "n.name", Op: "!=", Value: "Alice"},
			func() {
				_, _ = engine.AddNode("User", map[string]any{"name": "Alice"})
				_, _ = engine.AddNode("User", map[string]any{"name": "Bob"})
			},
			func(result *query.ResultSet) {
				Expect(result.Len()).To(Equal(1))
				row := result.Items()[0]
				node, ok := row["n"]
				Expect(ok).To(BeTrue())
				Expect(node.Props["name"]).To(Equal("Bob"))
			},
		)
	})

	Context("when filtering on number with >", func() {
		describeExecution("should respect > filter",
			q.Filter{Field: "n.age", Op: ">", Value: 31},
			func() {
				_, _ = engine.AddNode("User", map[string]any{"name": "Alice", "age": "35"})
				_, _ = engine.AddNode("User", map[string]any{"name": "Bob", "age": 30})
			},
			func(result *query.ResultSet) {
				Expect(result.Len()).To(Equal(1))
				name := result.Items()[0]["n"].Props["name"]
				Expect(name).To(Equal("Alice"))
			},
		)
	})

	Context("when filtering on number with <", func() {
		describeExecution("should respect > filter",
			q.Filter{Field: "n.age", Op: "<", Value: 31.0},
			func() {
				_, _ = engine.AddNode("User", map[string]any{"name": "Alice", "age": int64(35)})
				_, _ = engine.AddNode("User", map[string]any{"name": "Bob", "age": float32(30)})
			},
			func(result *query.ResultSet) {
				Expect(result.Len()).To(Equal(1))
				name := result.Items()[0]["n"].Props["name"]
				Expect(name).To(Equal("Bob"))
			},
		)
	})

	Context("with edge traversal", func() {
		It("should follow Has relationship", func() {
			// Register verb
			engine.RegisterVerb("has_skill", types.Verb{
				TargetLabel: "Skill",
				MatchOn:     "name",
			})

			// Create data
			aliceID, _ := engine.AddNode("User", map[string]any{"name": "Alice"})
			goID, _ := engine.AddNode("Skill", map[string]any{"name": "Go"})
			_ = engine.AddEdge(aliceID, goID, "has_skill", nil)

			// Build plan manually
			plan := &q.QueryPlan{
				Nodes: []*q.PatternNode{
					{Var: "n", Label: "User"},
					{Var: "v0", Label: "Skill"},
				},
				Edges: []*q.PatternEdge{
					{From: "n", To: "v0", Kind: "has_skill"},
				},
				Filters: []q.Filter{
					{Field: "v0.name", Op: "=", Value: "Go"},
				},
			}

			result, err := qe.Execute(context.Background(), storage, plan)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Len()).To(Equal(1))

			row := result.Items()[0]
			Expect(row).To(HaveKey("n"))
			Expect(row["n"].Props["name"]).To(Equal("Alice"))
		})
	})

	Context("with edge property filter", func() {
		It("should respect WhereEdge", func() {
			engine.RegisterVerb("contributes_to", types.Verb{
				TargetLabel: "Project",
				MatchOn:     "name",
			})

			userID1, _ := engine.AddNode("User", map[string]any{"name": "Alice"})
			userID2, _ := engine.AddNode("User", map[string]any{"name": "Bob"})
			projID, _ := engine.AddNode("Project", map[string]any{"name": "KnitKnot"})
			_ = engine.AddEdge(userID1, projID, "contributes_to", map[string]any{"level": 5})
			_ = engine.AddEdge(userID2, projID, "contributes_to", map[string]any{"level": 2})

			plan := &q.QueryPlan{
				Nodes: []*q.PatternNode{
					{Var: "n", Label: "User"},
					{Var: "v0", Label: "Project"},
				},
				Edges: []*q.PatternEdge{{
					From: "n", To: "v0", Kind: "contributes_to",
					Filters: []q.Filter{{Field: "level", Op: ">", Value: 3}},
				}},
				Filters: []q.Filter{
					{Field: "v0.name", Op: "=", Value: "KnitKnot"},
				},
			}

			// Simulate WhereEdge via manual edge filtering in engine
			// In real impl, edge filters are in PatternEdge.Filters
			result, err := qe.Execute(context.Background(), storage, plan)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Len()).To(Equal(1))
		})
	})
})
