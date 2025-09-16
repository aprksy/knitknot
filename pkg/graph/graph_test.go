package graph_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/aprksy/knitknot/pkg/graph"
	"github.com/aprksy/knitknot/pkg/ports/query"
	"github.com/aprksy/knitknot/pkg/ports/types"
	"github.com/aprksy/knitknot/pkg/storage/inmem"
)

var _ = Describe("Builder.Has", func() {
	var (
		engine  *graph.GraphEngine
		storage *inmem.Storage
		builder *graph.Builder
	)

	// Helper to extract plan without executing
	// buildPlan := func() *query.QueryPlan {
	// 	ctx := context.Background()
	// 	_, err := builder.Exec(ctx)
	// 	Expect(err).NotTo(HaveOccurred())
	// 	// In real impl, you might expose plan; here we assume it's accessible
	// 	// For demo, we'll verify via side effects
	// 	return nil // placeholder
	// }

	BeforeEach(func() {
		storage = inmem.New()
		engine = graph.NewGraphEngine(storage)

		// Register standard verb
		engine.RegisterVerb("has_skill", types.Verb{
			TargetLabel: "Skill",
			MatchOn:     "name",
		})

		builder = engine.Find("User")
	})

	Context("when using Has('has_skill', 'Go')", func() {
		It("should create a node pattern for Skill", func() {
			builder.Has("has_skill", "Go")

			plan := builder.Has("has_skill", "Go").ExportPlanForTest()
			Expect(plan.Nodes).To(ContainElement(&query.PatternNode{Var: "v0", Label: "Skill"}))

			// Simulate execution to capture plan
			result, err := builder.Exec(context.Background())
			Expect(err).NotTo(HaveOccurred())

			// We can't inspect plan directly unless exposed,
			// so let's test behavior instead: add real data and see if query matches

			// Add Alice (User)
			aliceID, err := engine.AddNode("User", map[string]any{"name": "Alice"})
			Expect(err).NotTo(HaveOccurred())

			// Add Go (Skill)
			goID, err := engine.AddNode("Skill", map[string]any{"name": "Go"})
			Expect(err).NotTo(HaveOccurred())

			// Connect them
			err = engine.AddEdge(aliceID, goID, "has_skill", nil)
			Expect(err).NotTo(HaveOccurred())

			// Re-run same query
			result, err = engine.Find("User").Has("has_skill", "Go").Limit(1).Exec(context.Background())
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Len()).To(Equal(1))

			row := result.Items()[0]
			n, ok := row["n"]
			Expect(ok).To(BeTrue())
			Expect(n.Label).To(Equal("User"))
			Expect(n.Props["name"]).To(Equal("Alice"))
		})
	})

	Context("when verb is unknown", func() {
		It("should fall back to Entity with name match", func() {
			builder.Has("likes", "Rust")

			// Add matching data
			userID, _ := engine.AddNode("User", map[string]any{"name": "Bob"})
			topicID, _ := engine.AddNode("Entity", map[string]any{"name": "Rust"})
			_ = engine.AddEdge(userID, topicID, "likes", nil)

			result, err := engine.Find("User").Has("likes", "Rust").Exec(context.Background())
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Len()).To(Equal(1))
		})
	})

	Context("when edge has properties", func() {
		It("should respect WhereEdge filters", func() {
			engine.RegisterVerb("contributes_to", types.Verb{
				TargetLabel: "Project",
				MatchOn:     "name",
			})

			// Add data
			userID, _ := engine.AddNode("User", map[string]any{"name": "Alice"})
			projID, _ := engine.AddNode("Project", map[string]any{"name": "KnitKnot"})
			_ = engine.AddEdge(userID, projID, "contributes_to", map[string]any{
				"level": 5,
			})

			// Query with edge filter
			result, err := engine.Find("User").
				Has("contributes_to", "KnitKnot").
				WhereEdge("level", ">", 3).
				Exec(context.Background())

			Expect(err).NotTo(HaveOccurred())
			Expect(result.Len()).To(Equal(1))
		})
	})

	Context("when verb has empty MatchOn", func() {
		It("should default to 'name' property", func() {
			engine.RegisterVerb("knows", types.Verb{
				TargetLabel: "Person",
				// MatchOn intentionally omitted
			})

			personID, _ := engine.AddNode("Person", map[string]any{"name": "Bob"})
			userID, _ := engine.AddNode("User", map[string]any{"name": "Alice"})
			_ = engine.AddEdge(userID, personID, "knows", nil)

			result, err := engine.Find("User").Has("knows", "Bob").Exec(context.Background())
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Len()).To(Equal(1))
		})
	})

	Context("when verb uses mixed case", func() {
		It("should match case-sensitively", func() {
			engine.RegisterVerb("HAS_SKILL", types.Verb{
				TargetLabel: "Skill",
				MatchOn:     "name",
			})

			skillID, _ := engine.AddNode("Skill", map[string]any{"name": "Go"})
			userID, _ := engine.AddNode("User", map[string]any{"name": "Alice"})
			_ = engine.AddEdge(userID, skillID, "HAS_SKILL", nil)

			result, err := engine.Find("User").Has("HAS_SKILL", "Go").Exec(context.Background())
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Len()).To(Equal(1))

			// But lowercase should fail
			result, err = engine.Find("User").Has("has_skill", "Go").Exec(context.Background())
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Len()).To(Equal(0))
		})
	})

	Context("when no edge exists", func() {
		It("should return empty result, not error", func() {
			// Nodes exist, but no edge
			_, _ = engine.AddNode("User", map[string]any{"name": "Alice"})
			_, _ = engine.AddNode("Skill", map[string]any{"name": "Go"})

			result, err := engine.Find("User").Has("has_skill", "Go").Exec(context.Background())
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Empty()).To(BeTrue())
			Expect(result.Len()).To(Equal(0))
		})
	})

	Context("when target node lacks property", func() {
		It("should not match", func() {
			userID, _ := engine.AddNode("User", map[string]any{"name": "Alice"})
			skillID, _ := engine.AddNode("Skill", map[string]any{}) // no 'name'
			_ = engine.AddEdge(userID, skillID, "has_skill", nil)

			result, err := engine.Find("User").Has("has_skill", "Go").Exec(context.Background())
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Empty()).To(BeTrue())
		})
	})

	Context("with multiple Has calls", func() {
		It("should bind distinct variables", func() {
			engine.RegisterVerb("teaches", types.Verb{TargetLabel: "Course", MatchOn: "code"})

			// Alice → teaches → CS101
			aliceID, _ := engine.AddNode("User", map[string]any{"name": "Alice"})
			cs101ID, _ := engine.AddNode("Course", map[string]any{"code": "CS101"})
			_ = engine.AddEdge(aliceID, cs101ID, "teaches", nil)

			// Alice → has_skill → Go
			goID, _ := engine.AddNode("Skill", map[string]any{"name": "Go"})
			_ = engine.AddEdge(aliceID, goID, "has_skill", nil)

			result, err := engine.Find("User").
				Has("teaches", "CS101").
				Has("has_skill", "Go").
				Exec(context.Background())

			Expect(err).NotTo(HaveOccurred())
			Expect(result.Len()).To(Equal(1))

			row := result.Items()[0]
			Expect(row).To(HaveKey("n"))  // User
			Expect(row).To(HaveKey("v0")) // Course
			Expect(row).To(HaveKey("v1")) // Skill
		})
	})
})
