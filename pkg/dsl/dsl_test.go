// cmd/dsl/parser_test.go
package dsl_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/aprksy/knitknot/cmd"
	"github.com/aprksy/knitknot/pkg/dsl"
	"github.com/aprksy/knitknot/pkg/graph"
	"github.com/aprksy/knitknot/pkg/ports/types"
	"github.com/aprksy/knitknot/pkg/storage/inmem"
)

var _ = Describe("DSL Parser", func() {
	parse := func(input string) (*dsl.Query, error) {
		p := dsl.NewParser(input)
		return p.Parse()
	}

	Describe("Parse Method Calls", func() {
		It("should parse Find('User')", func() {
			ast, err := parse("Find('User')")
			Expect(err).NotTo(HaveOccurred())
			Expect(ast).NotTo(BeNil())
			Expect(ast.Methods).To(HaveLen(1))

			method := ast.Methods[0]
			Expect(method.Name.Value).To(Equal("Find"))
			Expect(method.Arguments).To(HaveLen(1))
			arg, ok := method.Arguments[0].(*dsl.StringLiteral)
			Expect(ok).To(BeTrue())
			Expect(arg.Value).To(Equal("User"))
		})

		It("should parse Has('has_skill', 'Go')", func() {
			ast, err := parse("Has('has_skill', 'Go')")
			Expect(err).NotTo(HaveOccurred())
			Expect(ast.Methods).To(HaveLen(1))

			method := ast.Methods[0]
			Expect(method.Name.Value).To(Equal("Has"))
			Expect(method.Arguments).To(HaveLen(2))

			relArg, ok := method.Arguments[0].(*dsl.StringLiteral)
			Expect(ok).To(BeTrue())
			Expect(relArg.Value).To(Equal("has_skill"))

			valArg, ok := method.Arguments[1].(*dsl.StringLiteral)
			Expect(ok).To(BeTrue())
			Expect(valArg.Value).To(Equal("Go"))
		})

		It("should parse Where('n.age', '>', 30)", func() {
			ast, err := parse("Where('n.age', '>', 30)")
			Expect(err).NotTo(HaveOccurred())
			Expect(ast.Methods).To(HaveLen(1))

			method := ast.Methods[0]
			Expect(method.Name.Value).To(Equal("Where"))
			Expect(method.Arguments).To(HaveLen(3))

			fieldArg, ok := method.Arguments[0].(*dsl.StringLiteral)
			Expect(ok).To(BeTrue())
			Expect(fieldArg.Value).To(Equal("n.age"))

			opArg, ok := method.Arguments[1].(*dsl.StringLiteral)
			Expect(ok).To(BeTrue())
			Expect(opArg.Value).To(Equal(">"))

			numArg, ok := method.Arguments[2].(*dsl.NumberLiteral)
			Expect(ok).To(BeTrue())
			Expect(numArg.Value).To(Equal(30))
		})

		It("should parse Limit(5)", func() {
			ast, err := parse("Limit(5)")
			Expect(err).NotTo(HaveOccurred())
			Expect(ast.Methods).To(HaveLen(1))

			method := ast.Methods[0]
			Expect(method.Name.Value).To(Equal("Limit"))
			numArg, ok := method.Arguments[0].(*dsl.NumberLiteral)
			Expect(ok).To(BeTrue())
			Expect(numArg.Value).To(Equal(5))
		})
	})

	Describe("Chained Queries", func() {
		It("should parse Find('User').Has('has_skill', 'Go')", func() {
			ast, err := parse("Find('User').Has('has_skill', 'Go')")
			Expect(err).NotTo(HaveOccurred())
			Expect(ast.Methods).To(HaveLen(2))

			Expect(ast.Methods[0].Name.Value).To(Equal("Find"))
			Expect(ast.Methods[1].Name.Value).To(Equal("Has"))
		})

		It("should parse complex chain with Where and Limit", func() {
			ast, err := parse("Find('User').Has('has_skill', 'Go').Where('n.age', '>', 35).Limit(10)")
			Expect(err).NotTo(HaveOccurred())
			Expect(ast.Methods).To(HaveLen(4))

			Expect(ast.Methods[0].Name.Value).To(Equal("Find"))
			Expect(ast.Methods[1].Name.Value).To(Equal("Has"))
			Expect(ast.Methods[2].Name.Value).To(Equal("Where"))
			Expect(ast.Methods[3].Name.Value).To(Equal("Limit"))
		})
	})

	Describe("Whitespace Handling", func() {
		It("should ignore extra whitespace", func() {
			ast1, _ := parse("Find('User')")
			ast2, _ := parse("  Find  (  'User'  )  ")
			Expect(ast1.Methods[0].Name.Value).To(Equal(ast2.Methods[0].Name.Value))

			// Just validate structure survives spacing
			Expect(ast2.Methods).To(HaveLen(1))
			arg, ok := ast2.Methods[0].Arguments[0].(*dsl.StringLiteral)
			Expect(ok).To(BeTrue())
			Expect(arg.Value).To(Equal("User"))
		})

		It("should handle newlines and tabs", func() {
			input := "Find('User')\n\t.Has('has_skill', 'Go')"
			ast, err := parse(input)
			Expect(err).NotTo(HaveOccurred())
			Expect(ast.Methods).To(HaveLen(2))
			Expect(ast.Methods[0].Name.Value).To(Equal("Find"))
			Expect(ast.Methods[1].Name.Value).To(Equal("Has"))
		})
	})

	Describe("Error Cases", func() {
		It("should reject missing closing parenthesis", func() {
			_, err := parse("Find('User'")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("expected RPAREN, got "))
		})

		It("should reject unmatched quotes", func() {
			_, err := parse("Find('User)")
			Expect(err).To(HaveOccurred())
		})

		It("should reject missing comma", func() {
			_, err := parse("Has('has_skill' 'Go')")
			Expect(err).To(HaveOccurred())
		})

		It("should reject empty method name", func() {
			_, err := parse(".Has('x','y')")
			Expect(err).To(HaveOccurred())
		})

		It("should reject dangling dot", func() {
			_, err := parse("Find('User').")
			Expect(err).To(HaveOccurred())
		})

		It("should reject method without LPAREN", func() {
			_, err := parse("Find.Limit(10)")
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("Integration with Query Engine", func() {
		It("should produce a valid QueryPlan when executed", func() {
			storage := inmem.New()
			engine := graph.NewGraphEngine(storage)
			engine.RegisterVerb("has_skill", types.Verb{TargetLabel: "Skill", MatchOn: "name"})

			// Add sample data
			aliceID, _ := engine.AddNode("User", map[string]any{"name": "Alice", "age": 40})
			goID, _ := engine.AddNode("Skill", map[string]any{"name": "Go"})
			_ = engine.AddEdge(aliceID, goID, "has_skill", nil)

			// Parse and execute
			ast, err := parse("Find('User').Has('has_skill', 'Go').Where('n.age', '>', 30)")
			Expect(err).NotTo(HaveOccurred())

			builder, err := cmd.ApplyAST(engine, ast)
			Expect(err).NotTo(HaveOccurred())

			result, err := builder.Exec(context.Background())
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Len()).To(Equal(1))
		})
	})
})
