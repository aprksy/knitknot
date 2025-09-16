package inmem_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/aprksy/knitknot/pkg/graph"
	"github.com/aprksy/knitknot/pkg/ports/types"
	"github.com/aprksy/knitknot/pkg/storage/inmem"
)

var _ = Describe("In-Memory Storage Persistence", func() {
	var (
		storage  *inmem.Storage
		tmpDir   string
		filename string
		engine   *graph.GraphEngine
		n1, n2   string
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "knitknot-test-*")
		Expect(err).NotTo(HaveOccurred())

		filename = filepath.Join(tmpDir, "test.gob")
		storage = inmem.New()
		engine = graph.NewGraphEngine(storage)

		// Add test data
		n1, _ = storage.AddNode("User", map[string]any{"name": "Alice"})
		node1, _ := storage.GetNode(n1)
		storage.AddToSubgraph(node1, "common-subgraph", "")
		storage.AddToSubgraph(node1, "node1-only", "")

		n2, _ = storage.AddNode("Skill", map[string]any{"name": "Go"})
		node2, _ := storage.GetNode(n2)
		storage.AddToSubgraph(node2, "common-subgraph", "")
		storage.AddToSubgraph(node2, "node2-only", "")

		_ = storage.AddEdge(n1, n2, "has_skill", map[string]any{"level": 4})

	})

	AfterEach(func() {
		os.RemoveAll(tmpDir)
	})

	Describe("Save and Load", func() {
		It("should survive roundtrip", func() {
			// Save
			err := storage.Save(filename, engine)
			Expect(err).NotTo(HaveOccurred())

			// Load into new storage
			newStorage := inmem.New()
			err = newStorage.Load(filename, nil) // no engine → skip verbs
			Expect(err).NotTo(HaveOccurred())

			Expect(len(newStorage.GetAllNodes())).To(Equal(2))
			Expect(len(newStorage.GetAllEdges())).To(Equal(1))

			// Verify nodes
			node1, ok := newStorage.GetNode(n1)
			Expect(ok).To(BeTrue())
			Expect(node1.Label).To(Equal("User"))
			Expect(node1.Props["name"]).To(Equal("Alice"))

			node2, ok := newStorage.GetNode(n2)
			Expect(ok).To(BeTrue())
			Expect(node2.Label).To(Equal("Skill"))

			// Verify edges
			edges := newStorage.GetEdgesFrom(n1)
			Expect(edges).To(HaveLen(1))
			e := edges[0]
			Expect(e.Kind).To(Equal("has_skill"))
			Expect(e.Props["level"]).To(Equal(4))
		})

		It("should preserve verb registry when loaded with engine", func() {
			// Setup engine with verbs
			engine := graph.NewGraphEngine(storage)
			engine.RegisterVerb("has_skill", types.Verb{
				TargetLabel: "Skill",
				MatchOn:     "name",
			})

			// Save
			Expect(storage.Save(filename, engine)).To(Succeed())

			// Load with engine
			newStorage := inmem.New()
			newEngine := graph.NewGraphEngine(newStorage)
			Expect(newStorage.Load(filename, newEngine)).To(Succeed())

			// Check verbs
			verbs := newEngine.Verbs().All()
			verb, ok := verbs["has_skill"]
			Expect(ok).To(BeTrue())
			Expect(verb.TargetLabel).To(Equal("Skill"))
			Expect(verb.MatchOn).To(Equal("name"))

			Expect(len(storage.GetNodesIn("common-subgraph"))).To(Equal(2))
			Expect(len(storage.GetNodesIn("node1-only"))).To(Equal(1))
			Expect(len(storage.GetNodesIn("node2-only"))).To(Equal(1))
			Expect(len(storage.GetEdgesIn("common-subgraph"))).To(Equal(1))
		})

		It("should handle missing file gracefully", func() {
			err := (&inmem.Storage{}).Load("not-there.gob", nil)
			Expect(err).To(HaveOccurred())
		})

		It("should create dir if needed", func() {
			deepFile := filepath.Join(tmpDir, "subdir", "deep.gob")
			err := storage.Save(deepFile, engine)
			Expect(err).NotTo(HaveOccurred())

			info, err := os.Stat(deepFile)
			Expect(err).NotTo(HaveOccurred())
			Expect(info.Size()).To(BeNumerically(">", 0))
		})
	})
})
