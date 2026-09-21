// L4 regression: parseProps keeps spaced values (via execAddNode; parseProps is unexported).
package cmd

import (
	"bytes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/aprksy/knitknot/pkg/graph"
	"github.com/aprksy/knitknot/pkg/storage/inmem"
)

var _ = Describe("execAddNode", func() {
	var (
		engine *graph.GraphEngine
		saved  struct {
			subgraph string
			file     string
		}
	)

	findPerson := func() map[string]any {
		st := engine.Storage().(*inmem.Storage)
		for _, n := range st.GetAllNodes() {
			if n.Label == "Person" {
				return n.Props
			}
		}
		return nil
	}

	BeforeEach(func() {
		saved = globalFlags
		engine = graph.NewGraphEngine(inmem.New())
	})

	AfterEach(func() {
		globalFlags = saved
	})

	It("L4: captures multi-word values", func() {
		var out bytes.Buffer
		Expect(execAddNode(engine, "Person name=John Doe", &out)).To(Succeed())
		Expect(findPerson()["name"]).To(Equal("John Doe"))
	})

	It("L4: coerces ints, leaves bools as strings", func() {
		var out bytes.Buffer
		Expect(execAddNode(engine, "Person age=35 active=true", &out)).To(Succeed())
		props := findPerson()
		Expect(props["age"]).To(Equal(35))
		Expect(props["active"]).To(Equal("true"))
	})
})
