// Locks down H1: exit/quit route through the errExitRepl sentinel (not os.Exit)
// so autosave runs; plus routing of every handleLine command prefix.
package cmd

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/aprksy/knitknot/pkg/graph"
	"github.com/aprksy/knitknot/pkg/storage/inmem"
)

func newDispatchEngine() *graph.GraphEngine {
	return graph.NewGraphEngine(inmem.New())
}

var _ = Describe("handleLine dispatch", func() {
	var (
		ctx    context.Context
		engine *graph.GraphEngine
		out    *bytes.Buffer
	)

	BeforeEach(func() {
		ctx = context.Background()
		engine = newDispatchEngine()
		out = &bytes.Buffer{}
	})

	Describe("H1: exit sentinel routing", func() {
		It("returns errExitRepl for 'exit'", func() {
			err := handleLine(ctx, "exit", engine, out)
			Expect(err).To(HaveOccurred())
			Expect(errors.Is(err, errExitRepl)).To(BeTrue(), "exit must return the errExitRepl sentinel")
		})

		It("returns errExitRepl for 'quit'", func() {
			err := handleLine(ctx, "quit", engine, out)
			Expect(err).To(HaveOccurred())
			Expect(errors.Is(err, errExitRepl)).To(BeTrue(), "quit must return the errExitRepl sentinel")
		})

		It("routes uppercase 'EXIT' to exit (switch is on lowercased input)", func() {
			err := handleLine(ctx, "EXIT", engine, out)
			Expect(err).To(HaveOccurred())
			Expect(errors.Is(err, errExitRepl)).To(BeTrue())
		})

		It("routes 'exit ' with trailing whitespace to exit (TrimSpace)", func() {
			err := handleLine(ctx, "exit ", engine, out)
			Expect(err).To(HaveOccurred())
			Expect(errors.Is(err, errExitRepl)).To(BeTrue())
		})
	})

	Describe("help and verb listing", func() {
		It("routes 'help' with nil error", func() {
			Expect(handleLine(ctx, "help", engine, out)).To(Succeed())
			Expect(out.String()).To(ContainSubstring("REPL Commands"))
		})

		It("routes 'list verbs', 'verbs' and 'LIST VERBS' with nil error", func() {
			for _, input := range []string{"list verbs", "verbs", "LIST VERBS"} {
				buf := &bytes.Buffer{}
				Expect(handleLine(ctx, input, engine, buf)).To(Succeed(), "input %q should route to execListVerbs", input)
			}
		})
	})

	Describe("command prefix routing", func() {
		It("routes 'addnode ...' to execAddNode (creates a node)", func() {
			err := handleLine(ctx, "addnode User name=Bob", engine, out)
			Expect(err).To(Succeed())
			Expect(out.String()).To(ContainSubstring("Created node"))
		})

		It("routes 'connect ...' with bad syntax to execConnect usage error", func() {
			err := handleLine(ctx, "connect garbage", engine, out)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid format"))
		})

		It("routes 'connect ...' with real nodes to a successful edge", func() {
			id1, err := engine.AddNode("User", map[string]any{"name": "A"})
			Expect(err).To(Succeed())
			id2, err := engine.AddNode("User", map[string]any{"name": "B"})
			Expect(err).To(Succeed())
			err = handleLine(ctx, "connect "+id1+" --knows--> "+id2, engine, out)
			Expect(err).To(Succeed())
			Expect(out.String()).To(ContainSubstring("Connected"))
		})

		It("routes 'update ...' to execUpdate usage error", func() {
			err := handleLine(ctx, "update garbage", engine, out)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("usage: UPDATE"))
		})

		It("routes 'delete ...' to execDelete usage error", func() {
			err := handleLine(ctx, "delete garbage", engine, out)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("usage: DELETE"))
		})

		It("routes 'explain ...' to execExplain (valid query, nil error)", func() {
			err := handleLine(ctx, "explain Find('User')", engine, out)
			Expect(err).To(Succeed())
		})

		It("routes 'define ...' with bad syntax to execDefine usage error", func() {
			err := handleLine(ctx, "define garbage", engine, out)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid syntax"))
		})

		It("routes 'define ...' with valid syntax to a registered verb", func() {
			err := handleLine(ctx, "define likes to Skill via name", engine, out)
			Expect(err).To(Succeed())
			Expect(out.String()).To(ContainSubstring("likes"))
		})

		It("routes 'load ...' for a missing file to execLoad file-not-found error", func() {
			err := handleLine(ctx, "load /nonexistent-xyz.gob", engine, out)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("file not found"))
		})

		It("routes 'save ...' to execSave (writes the file)", func() {
			tmp := filepath.Join(os.TempDir(), "knitknot-dispatch-test.gob")
			DeferCleanup(os.Remove, tmp)
			err := handleLine(ctx, "save "+tmp, engine, out)
			Expect(err).To(Succeed())
			Expect(out.String()).To(ContainSubstring("Saved"))
		})
	})

	Describe("default routing", func() {
		It("sends unknown input to execQuery (parse error for non-DSL)", func() {
			err := handleLine(ctx, "blarg nonsense !!!", engine, out)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("parse error"))
		})
	})
})
