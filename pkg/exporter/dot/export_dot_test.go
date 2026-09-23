package dot_test

import (
	"bytes"
	"errors"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/aprksy/knitknot/pkg/exporter/dot"
	"github.com/aprksy/knitknot/pkg/ports/types"
)

// exp: dot-tests — specs locking down ExportToDOT behavior.

// exp: dot-tests — always fails; verifies write errors propagate.
type failWriter struct{ err error }

func (f failWriter) Write([]byte) (int, error) { return 0, f.err }

// exp: dot-tests — runs the exporter and returns the output.
func export(nodes []*types.Node, edges []*types.Edge) string {
	var buf bytes.Buffer
	Expect(dot.ExportToDOT(nodes, edges, &buf)).To(Succeed())
	return buf.String()
}

var _ = Describe("ExportToDOT", func() {
	It("wraps output in a digraph envelope", func() {
		out := export(nil, nil)
		Expect(out).To(HavePrefix("digraph KnitKnot {\n"))
		Expect(out).To(HaveSuffix("}\n"))
	})

	It("maps hyphens in IDs to underscores in node names", func() {
		out := export([]*types.Node{
			{ID: "n42", Label: "Person"},
			{ID: "indicator--abc-def", Label: "Person"},
		}, nil)
		Expect(out).To(ContainSubstring("N_n42 [label="))
		Expect(out).To(ContainSubstring("N_indicator__abc_def [label="))
	})

	It("uses the name prop in the node label", func() {
		out := export([]*types.Node{
			{ID: "n1", Label: "Person", Props: map[string]any{"name": "Alice"}},
		}, nil)
		Expect(out).To(ContainSubstring(`label="Person:Alice"`))
	})

	It("falls back to the title prop when name is absent", func() {
		out := export([]*types.Node{
			{ID: "n1", Label: "Person", Props: map[string]any{"title": "Dr"}},
		}, nil)
		Expect(out).To(ContainSubstring(`label="Person:Dr"`))
	})

	It("prefers name over title when both are present", func() {
		out := export([]*types.Node{
			{ID: "n1", Label: "Person", Props: map[string]any{"name": "Alice", "title": "Dr"}},
		}, nil)
		Expect(out).To(ContainSubstring(`label="Person:Alice"`))
	})

	It("uses the bare label when neither name nor title is present", func() {
		out := export([]*types.Node{
			{ID: "n1", Label: "Person", Props: map[string]any{"other": "x"}},
		}, nil)
		Expect(out).To(ContainSubstring(`label="Person"`))
		Expect(out).NotTo(ContainSubstring("Person:"))
	})

	It("emits an edge with mapped node names and kind label", func() {
		out := export([]*types.Node{
			{ID: "n1", Label: "A"},
			{ID: "n2", Label: "B"},
		}, []*types.Edge{
			{From: "n1", To: "n2", Kind: "rel"},
		})
		Expect(out).To(ContainSubstring(`N_n1 -> N_n2 [label="rel"];`))
	})

	It("dedups edges with the same from->to@kind", func() {
		out := export(nil, []*types.Edge{
			{From: "n1", To: "n2", Kind: "rel"},
			{From: "n1", To: "n2", Kind: "rel"},
		})
		Expect(strings.Count(out, `N_n1 -> N_n2 [label="rel"];`)).To(Equal(1))
	})

	It("emits both edges when kinds differ between the same nodes", func() {
		out := export(nil, []*types.Edge{
			{From: "n1", To: "n2", Kind: "rel"},
			{From: "n1", To: "n2", Kind: "other"},
		})
		Expect(out).To(ContainSubstring(`N_n1 -> N_n2 [label="rel"];`))
		Expect(out).To(ContainSubstring(`N_n1 -> N_n2 [label="other"];`))
	})

	It("emits a self-loop", func() {
		out := export(nil, []*types.Edge{
			{From: "n1", To: "n1", Kind: "rel"},
		})
		Expect(out).To(ContainSubstring(`N_n1 -> N_n1 [label="rel"];`))
	})

	It("emits just the envelope for an empty graph", func() {
		Expect(export(nil, nil)).To(Equal("digraph KnitKnot {\n}\n"))
	})

	It("matches a golden string for a small graph", func() {
		out := export([]*types.Node{
			{ID: "n1", Label: "Person", Props: map[string]any{"name": "Alice"}},
			{ID: "n2", Label: "Skill", Props: map[string]any{"title": "Go"}},
		}, []*types.Edge{
			{From: "n1", To: "n2", Kind: "has_skill"},
			{From: "n2", To: "n1", Kind: "taught_by"},
		})
		Expect(out).To(Equal("digraph KnitKnot {\n" +
			"  N_n1 [label=\"Person:Alice\", shape=box, style=rounded];\n" +
			"  N_n2 [label=\"Skill:Go\", shape=box, style=rounded];\n" +
			"  N_n1 -> N_n2 [label=\"has_skill\"];\n" +
			"  N_n2 -> N_n1 [label=\"taught_by\"];\n" +
			"}\n"))
	})

	It("Go-quotes labels containing quotes", func() {
		out := export([]*types.Node{
			{ID: "n1", Label: "Person", Props: map[string]any{"name": `Bob "the builder"`}},
		}, nil)
		Expect(out).To(ContainSubstring(`Bob \"the builder\"`))
	})

	It("propagates write errors", func() {
		want := errors.New("boom")
		err := dot.ExportToDOT([]*types.Node{{ID: "n1", Label: "A"}}, nil, failWriter{err: want})
		Expect(err).To(MatchError(want))
	})
})
