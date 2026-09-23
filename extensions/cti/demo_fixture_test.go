// ext: cti-demo-fixture — narrative demo-campaign bundle fixture tests over testdata/demo-campaign.json.
package cti_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/aprksy/knitknot/extensions"
	"github.com/aprksy/knitknot/extensions/cti"
	"github.com/aprksy/knitknot/pkg/ports/types"
	"github.com/aprksy/knitknot/pkg/storage/inmem"
)

const demoRevokedStixID = "indicator--39393939-3939-3939-3939-393939393939" // ext: cti-demo-fixture

func readDemoFixture(t *testing.T) []byte { // ext: cti-demo-fixture
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "demo-campaign.json"))
	if err != nil {
		t.Fatalf("read demo fixture: %v", err)
	}
	return data
}

func importDemo(t *testing.T, s *inmem.Storage, data []byte) { // ext: cti-demo-fixture
	t.Helper()
	if err := cti.NewSTIXImporter().Import(context.Background(), extension.ImportContext{}, s, types.NewVerbRegistry(), bytes.NewReader(data)); err != nil {
		t.Fatalf("import: %v", err)
	}
}

func TestDemoCampaignLoads(t *testing.T) { // ext: cti-demo-fixture
	s := inmem.New()
	importDemo(t, s, readDemoFixture(t))

	if got := len(s.GetAllNodes()); got != 15 {
		t.Fatalf("nodes = %d, want 15", got)
	}
	if got := len(s.GetAllEdges()); got != 10 {
		t.Fatalf("edges = %d, want 10", got)
	}
}

func TestDemoCampaignRevokedPreserved(t *testing.T) { // ext: cti-demo-fixture
	s := inmem.New()
	importDemo(t, s, readDemoFixture(t))

	for _, n := range s.GetAllNodes() {
		id, _ := n.Props[cti.StixID].(string)
		if id != demoRevokedStixID {
			continue
		}
		if v, _ := n.Props[cti.PropRevoked].(bool); v != true {
			t.Fatalf("revoked indicator %q prop revoked = %v, want true", id, n.Props[cti.PropRevoked])
		}
		return
	}
	t.Fatalf("revoked indicator %q not found", demoRevokedStixID)
}

func TestDemoCampaignIdempotent(t *testing.T) { // ext: cti-demo-fixture
	s := inmem.New()
	data := readDemoFixture(t)
	importDemo(t, s, data)
	importDemo(t, s, data)

	if got := len(s.GetAllNodes()); got != 15 {
		t.Fatalf("nodes = %d after reimport, want 15", got)
	}
	if got := len(s.GetAllEdges()); got != 10 {
		t.Fatalf("edges = %d after reimport, want 10 (no duplicate edges)", got)
	}
}
