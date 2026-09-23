package cmd

// version: history-cmd — specs for the generic history command.

import (
	"bytes"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/aprksy/knitknot/pkg/graph"
	"github.com/aprksy/knitknot/pkg/ports/types"
	"github.com/aprksy/knitknot/pkg/storage/inmem"
)

// version: history-cmd — stub backend without VersionedStorage.
type plainStorageStub struct{}

type storageEngineAlias = interface {
	AddNode(label string, props map[string]any) (string, error)
	AddEdge(from, to, kind string, props map[string]any) error
	GetNode(id string) (*types.Node, bool)
	GetEdge(id string) (*types.Edge, bool)
	GetAllNodes() []*types.Node
	GetNodesByLabel(label string) []*types.Node
	GetAllEdges() []*types.Edge
	GetEdgesFrom(from string) []*types.Edge
	GetEdgesTo(to string) []*types.Edge
	GetEdgesByKind(kind string) []*types.Edge
	GetNodesIn(subgraph string) []*types.Node
	GetEdgesIn(subgraph string) []*types.Edge
	UpdateNode(id string, props map[string]any) error
	UpdateEdge(id string, props map[string]any) error
	DeleteNode(id string) error
	DeleteEdge(from, to, kind string) error
}

func (p *plainStorageStub) AddNode(string, map[string]any) (string, error) { return "", nil }
func (p *plainStorageStub) AddEdge(string, string, string, map[string]any) error {
	return nil
}
func (p *plainStorageStub) GetNode(string) (*types.Node, bool) { return nil, false }
func (p *plainStorageStub) GetEdge(string) (*types.Edge, bool) { return nil, false }
func (p *plainStorageStub) GetAllNodes() []*types.Node         { return nil }
func (p *plainStorageStub) GetNodesByLabel(string) []*types.Node {
	return nil
}
func (p *plainStorageStub) GetAllEdges() []*types.Edge        { return nil }
func (p *plainStorageStub) GetEdgesFrom(string) []*types.Edge { return nil }
func (p *plainStorageStub) GetEdgesTo(string) []*types.Edge   { return nil }
func (p *plainStorageStub) GetEdgesByKind(string) []*types.Edge {
	return nil
}
func (p *plainStorageStub) GetNodesIn(string) []*types.Node         { return nil }
func (p *plainStorageStub) GetEdgesIn(string) []*types.Edge         { return nil }
func (p *plainStorageStub) UpdateNode(string, map[string]any) error { return nil }
func (p *plainStorageStub) UpdateEdge(string, map[string]any) error { return nil }
func (p *plainStorageStub) DeleteNode(string) error                 { return nil }
func (p *plainStorageStub) DeleteEdge(string, string, string) error { return nil }

var _ = Describe("history command", func() {
	It("missing arg returns usage error", func() {
		err := runHistory(historyCmd, []string{})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("usage: knitknot history <node-id> -f <file.gob>"))
	})

	It("unknown ID returns no-history error", func() {
		engine := graph.NewGraphEngine(inmem.New())
		_, err := getHistory(engine, "nope")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("no history"))
	})

	It("happy path prints header + 2 snapshots with sorted props", func() {
		engine := graph.NewGraphEngine(inmem.New())
		id, err := engine.AddNode("Thing", map[string]any{"zebra": "z", "apple": "a"})
		Expect(err).NotTo(HaveOccurred())
		Expect(engine.UpdateNode(id, map[string]any{"mango": "m", "apple": "a2"})).To(Succeed())

		snaps, err := getHistory(engine, id)
		Expect(err).NotTo(HaveOccurred())
		Expect(snaps).To(HaveLen(2))

		var buf bytes.Buffer
		printNodeHistory(&buf, id, snaps)
		lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
		Expect(lines).To(HaveLen(3))
		Expect(lines[0]).To(Equal("-- History for " + id + " (2 snapshots)"))
		Expect(lines[1]).To(ContainSubstring("rev="))
		Expect(lines[1]).To(ContainSubstring("props={apple=a, zebra=z}"))
		Expect(lines[2]).To(ContainSubstring("props={apple=a2, mango=m}"))
	})

	It("non-versioned backend returns honest error", func() {
		engine := graph.NewGraphEngine(&plainStorageStub{})
		_, err := getHistory(engine, "whatever")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("history not supported"))
	})
})
