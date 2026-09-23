// ext: cti-fixture — fixture-driven STIX 2.1 round-trip tests over testdata/minimal-bundle.json.
package cti_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/aprksy/knitknot/extensions"
	"github.com/aprksy/knitknot/extensions/cti"
	"github.com/aprksy/knitknot/pkg/ports/types"
	"github.com/aprksy/knitknot/pkg/storage/inmem"
)

var stixIDPattern = regexp.MustCompile(`^[a-z-]+--[0-9a-f-]{36}$`) // ext: cti-fixture

func readFixture(t *testing.T) []byte { // ext: cti-fixture
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "minimal-bundle.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return data
}

func importBytes(t *testing.T, s *inmem.Storage, data []byte) { // ext: cti-fixture
	t.Helper()
	if err := cti.NewSTIXImporter().Import(context.Background(), extension.ImportContext{}, s, types.NewVerbRegistry(), bytes.NewReader(data)); err != nil {
		t.Fatalf("import: %v", err)
	}
}

func TestFixtureRoundTrip(t *testing.T) { // ext: cti-fixture
	s := inmem.New()
	importBytes(t, s, readFixture(t))

	if got := len(s.GetAllNodes()); got != 6 {
		t.Fatalf("nodes = %d, want 6", got)
	}
	if got := len(s.GetAllEdges()); got != 3 {
		t.Fatalf("edges = %d, want 3", got)
	}

	if got := len(s.GetNodesByLabel(cti.LabelMalware)); got != 1 {
		t.Fatalf("malware nodes = %d, want 1", got)
	}
	indicators := s.GetNodesByLabel(cti.LabelIndicator)
	if len(indicators) != 1 {
		t.Fatalf("indicator nodes = %d, want 1", len(indicators))
	}

	for _, label := range []string{cti.LabelThreatActor, cti.LabelIdentity, cti.LabelVulnerability} {
		if got := len(s.GetNodesByLabel(label)); got != 1 {
			t.Fatalf("%s nodes = %d, want 1", label, got)
		}
	}

	custom := s.GetNodesByLabel("x-custom-thing")
	if len(custom) != 1 {
		t.Fatalf("x-custom-thing nodes = %d, want 1", len(custom))
	}
	if v, _ := custom[0].Props["custom_prop"].(string); v != "preserved-for-round-trip" {
		t.Fatalf("custom_prop = %q, want preserved-for-round-trip", v)
	}

	for _, n := range s.GetAllNodes() {
		id, _ := n.Props[cti.StixID].(string)
		if !stixIDPattern.MatchString(id) {
			t.Fatalf("node %q has bad stix_id %q", n.ID, id)
		}
		if v, _ := n.Props[cti.ImportedAt].(string); v == "" {
			t.Fatalf("node %q (%s) missing imported_at", n.ID, id)
		}
	}

	found := false
	for _, e := range s.GetEdgesFrom(indicators[0].ID) {
		if e.Kind == "indicates" {
			found = true
		}
	}
	if !found {
		t.Fatal("no indicates edge from indicator node")
	}
}

func TestFixtureIdempotentReimport(t *testing.T) { // ext: cti-fixture
	s := inmem.New()
	data := readFixture(t)
	importBytes(t, s, data)
	importBytes(t, s, data)

	if got := len(s.GetAllNodes()); got != 6 {
		t.Fatalf("nodes = %d after reimport, want 6", got)
	}
	if got := len(s.GetAllEdges()); got != 3 {
		t.Fatalf("edges = %d after reimport, want 3 (no duplicate edges)", got)
	}
}

func TestFixtureWholesomeDanglingRef(t *testing.T) { // ext: cti-fixture
	s := inmem.New()
	bad := `{
  "type": "bundle",
  "id": "bundle--aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
  "spec_version": "2.1",
  "objects": [
    {"type": "indicator", "spec_version": "2.1",
     "id": "indicator--bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
     "created": "2024-01-01T00:00:00.000Z", "modified": "2024-01-01T00:00:00.000Z",
     "pattern": "[domain-name:value = 'evil.example.com']",
     "pattern_type": "stix", "valid_from": "2024-01-01T00:00:00.000Z"},
    {"type": "relationship", "spec_version": "2.1",
     "id": "relationship--cccccccc-cccc-cccc-cccc-cccccccccccc",
     "created": "2024-01-02T00:00:00.000Z", "modified": "2024-01-02T00:00:00.000Z",
     "relationship_type": "indicates",
     "source_ref": "indicator--bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
     "target_ref": "malware--dddddddd-dddd-dddd-dddd-dddddddddddd"}
  ]
}`
	if err := cti.NewSTIXImporter().Import(context.Background(), extension.ImportContext{}, s, types.NewVerbRegistry(), strings.NewReader(bad)); err == nil {
		t.Fatal("expected error for dangling relationship target")
	}
	if got := len(s.GetAllNodes()); got != 0 {
		t.Fatalf("nodes = %d after failed import, want 0", got)
	}
	if got := len(s.GetAllEdges()); got != 0 {
		t.Fatalf("edges = %d after failed import, want 0", got)
	}
}
