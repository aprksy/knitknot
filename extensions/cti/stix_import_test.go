package cti_test

import (
	"context"
	"strings"
	"testing"

	"github.com/aprksy/knitknot/extensions"
	"github.com/aprksy/knitknot/extensions/cti"
	"github.com/aprksy/knitknot/pkg/ports/types"
	"github.com/aprksy/knitknot/pkg/storage/inmem"
)

const (
	indID = "indicator--11111111-1111-1111-1111-111111111111"
	malID = "malware--22222222-2222-2222-2222-222222222222"
	relID = "relationship--33333333-3333-3333-3333-333333333333"
)

// Relationship listed BEFORE its targets: import must not depend on order.
const bundleRelFirst = `{
  "type": "bundle",
  "id": "bundle--44444444-4444-4444-4444-444444444444",
  "objects": [
    {"type": "relationship", "spec_version": "2.1", "id": "` + relID + `",
     "created": "2024-01-02T00:00:00.000Z", "modified": "2024-01-02T00:00:00.000Z",
     "relationship_type": "indicates", "source_ref": "` + indID + `", "target_ref": "` + malID + `"},
    {"type": "indicator", "spec_version": "2.1", "id": "` + indID + `",
     "created": "2024-01-01T00:00:00.000Z", "modified": "2024-01-01T00:00:00.000Z",
     "name": "bad domain", "pattern": "[domain-name:value = 'evil.example.com']",
     "pattern_type": "stix", "valid_from": "2024-01-01T00:00:00.000Z"},
    {"type": "malware", "spec_version": "2.1", "id": "` + malID + `",
     "created": "2024-01-01T00:00:00.000Z", "modified": "2024-01-01T00:00:00.000Z",
     "name": "EvilWare", "is_family": false, "malware_types": ["ransomware"]}
  ]
}`

func stixOf(n *types.Node) string {
	v, _ := n.Props[cti.StixID].(string)
	return v
}

func TestImportRelFirst(t *testing.T) {
	s := inmem.New()
	if err := cti.NewSTIXImporter().Import(context.Background(), s, types.NewVerbRegistry(), strings.NewReader(bundleRelFirst)); err != nil {
		t.Fatalf("import: %v", err)
	}
	if got := len(s.GetAllNodes()); got != 2 {
		t.Fatalf("nodes = %d, want 2", got)
	}
	edges := s.GetAllEdges()
	if len(edges) != 1 {
		t.Fatalf("edges = %d, want 1", len(edges))
	}
	e := edges[0]
	if e.Kind != "indicates" {
		t.Fatalf("edge kind = %q, want indicates", e.Kind)
	}
	ends := map[string]bool{}
	for _, id := range []string{e.From, e.To} {
		n, ok := s.GetNode(id)
		if !ok {
			t.Fatalf("edge endpoint %q not found", id)
		}
		ends[stixOf(n)] = true
	}
	if !ends[indID] || !ends[malID] {
		t.Fatalf("edge endpoints = %v, want indicator+malware", ends)
	}
}

func TestImportIdempotent(t *testing.T) {
	s := inmem.New()
	imp := cti.NewSTIXImporter()
	vr := types.NewVerbRegistry()
	for i := 0; i < 2; i++ {
		if err := imp.Import(context.Background(), s, vr, strings.NewReader(bundleRelFirst)); err != nil {
			t.Fatalf("import %d: %v", i, err)
		}
	}
	if got := len(s.GetAllNodes()); got != 2 {
		t.Fatalf("nodes = %d, want 2", got)
	}
	if got := len(s.GetAllEdges()); got != 1 {
		t.Fatalf("edges = %d, want 1", got)
	}
}

func TestImportDanglingRefNoPartialMutation(t *testing.T) {
	s := inmem.New()
	bad := `{
  "type": "bundle",
  "id": "bundle--55555555-5555-5555-5555-555555555555",
  "objects": [
    {"type": "indicator", "spec_version": "2.1", "id": "` + indID + `",
     "created": "2024-01-01T00:00:00.000Z", "modified": "2024-01-01T00:00:00.000Z",
     "pattern": "[domain-name:value = 'evil.example.com']",
     "pattern_type": "stix", "valid_from": "2024-01-01T00:00:00.000Z"},
    {"type": "relationship", "spec_version": "2.1", "id": "` + relID + `",
     "created": "2024-01-02T00:00:00.000Z", "modified": "2024-01-02T00:00:00.000Z",
     "relationship_type": "indicates", "source_ref": "` + indID + `",
     "target_ref": "malware--99999999-9999-9999-9999-999999999999"}
  ]
}`
	if err := cti.NewSTIXImporter().Import(context.Background(), s, types.NewVerbRegistry(), strings.NewReader(bad)); err == nil {
		t.Fatal("expected error for dangling relationship target")
	}
	if got := len(s.GetAllNodes()); got != 0 {
		t.Fatalf("nodes = %d after failed import, want 0 (no partial mutation)", got)
	}
	if got := len(s.GetAllEdges()); got != 0 {
		t.Fatalf("edges = %d after failed import, want 0", got)
	}
}

func TestImportCustomObjectPreserved(t *testing.T) {
	s := inmem.New()
	customID := "x-custom-thing--66666666-6666-6666-6666-666666666666"
	bundle := `{
  "type": "bundle",
  "id": "bundle--77777777-7777-7777-7777-777777777777",
  "objects": [
    {"type": "x-custom-thing", "spec_version": "2.1", "id": "` + customID + `",
     "created": "2024-01-01T00:00:00.000Z", "modified": "2024-01-01T00:00:00.000Z",
     "name": "weird", "custom_prop": "kept"}
  ]
}`
	if err := cti.NewSTIXImporter().Import(context.Background(), s, types.NewVerbRegistry(), strings.NewReader(bundle)); err != nil {
		t.Fatalf("import: %v", err)
	}
	nodes := s.GetAllNodes()
	if len(nodes) != 1 {
		t.Fatalf("nodes = %d, want 1", len(nodes))
	}
	n := nodes[0]
	if typ, _ := n.Props[cti.PropType].(string); typ != "x-custom-thing" {
		t.Fatalf("type prop = %q, want x-custom-thing", typ)
	}
	if v, _ := n.Props["custom_prop"].(string); v != "kept" {
		t.Fatalf("custom_prop = %q, want kept", v)
	}
	if stixOf(n) != customID {
		t.Fatalf("stix_id = %q, want %q", stixOf(n), customID)
	}
}

type fakeRegistry struct {
	verbs     map[string]types.Verb
	importers map[string]extension.Importer
}

func (f *fakeRegistry) RegisterVerb(name string, v types.Verb) error {
	f.verbs[name] = v
	return nil
}

func (f *fakeRegistry) RegisterImporter(name string, i extension.Importer) error {
	f.importers[name] = i
	return nil
}

func (f *fakeRegistry) RegisterExporter(name string, e extension.Exporter) error { return nil }

func (f *fakeRegistry) Importer(name string) (extension.Importer, bool) {
	i, ok := f.importers[name]
	return i, ok
}

func (f *fakeRegistry) Exporter(name string) (extension.Exporter, bool) { return nil, false }

func (f *fakeRegistry) Freeze() {}

func (f *fakeRegistry) importer(name string) (extension.Importer, bool) {
	return f.Importer(name)
}

func (f *fakeRegistry) verb(name string) (types.Verb, bool) {
	v, ok := f.verbs[name]
	return v, ok
}

func TestExtensionRegisters(t *testing.T) {
	r := &fakeRegistry{verbs: map[string]types.Verb{}, importers: map[string]extension.Importer{}}
	if err := cti.New().Register(r); err != nil {
		t.Fatalf("register: %v", err)
	}
	if _, ok := r.importer("stix-2.1"); !ok {
		t.Fatal("importer stix-2.1 not registered")
	}
	for _, kind := range []string{"uses", "targets", "indicates", "attributed-to", "communicates-with", "based-on", "derived-from", "object-ref"} {
		v, ok := r.verb(kind)
		if !ok {
			t.Fatalf("verb %q not registered", kind)
		}
		if v.MatchOn != "name" || v.TargetLabel == "" {
			t.Fatalf("verb %q = %+v, want MatchOn=name and non-empty target", kind, v)
		}
	}
}
