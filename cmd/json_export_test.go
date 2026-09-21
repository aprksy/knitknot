// Locks down M7: runExport --format json produces valid JSON (was a
// silent no-op, then a "not implemented" error). Seeded graph gob ->
// runExport -> parse the written file, assert the node survives.
package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/aprksy/knitknot/pkg/graph"
	"github.com/aprksy/knitknot/pkg/storage/inmem"
)

var _ = Describe("export --format json (M7)", func() {
	var (
		origFile, origSubgraph string
		origFormat, origOutput string
		tmpDir                 string
		gobFile, jsonOut       string
	)

	BeforeEach(func() {
		origFile, origSubgraph = globalFlags.file, globalFlags.subgraph
		origFormat, origOutput = exportFlags.format, exportFlags.output

		var err error
		tmpDir, err = os.MkdirTemp("", "knitknot-json-test-*")
		Expect(err).NotTo(HaveOccurred())
		gobFile = filepath.Join(tmpDir, "seed.gob")
		jsonOut = filepath.Join(tmpDir, "out.json")

		// Seed a tiny graph on disk so runExport has something to export.
		s := inmem.New()
		engine := graph.NewGraphEngine(s)
		_, err = engine.AddNode("User", map[string]any{"name": "Alice"})
		Expect(err).NotTo(HaveOccurred())
		Expect(s.Save(gobFile, engine)).To(Succeed())

		globalFlags.file = gobFile
		globalFlags.subgraph = ""
		exportFlags.format = "json"
		exportFlags.output = jsonOut
	})

	AfterEach(func() {
		globalFlags.file, globalFlags.subgraph = origFile, origSubgraph
		exportFlags.format, exportFlags.output = origFormat, origOutput
		os.RemoveAll(tmpDir)
	})

	It("writes valid JSON containing the seeded node", func() {
		Expect(runExport(nil, nil)).To(Succeed())

		data, err := os.ReadFile(jsonOut)
		Expect(err).NotTo(HaveOccurred())

		var doc struct {
			Nodes []struct {
				ID    string         `json:"id"`
				Label string         `json:"label"`
				Props map[string]any `json:"props"`
			} `json:"nodes"`
		}
		Expect(json.Unmarshal(data, &doc)).To(Succeed())
		Expect(doc.Nodes).To(HaveLen(1))
		Expect(doc.Nodes[0].Label).To(Equal("User"))
		Expect(doc.Nodes[0].Props["name"]).To(Equal("Alice"))
	})
})
