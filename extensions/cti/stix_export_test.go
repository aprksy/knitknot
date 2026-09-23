// ext: cti-export — STIX 2.1 exporter tests (stdlib testing only).
package cti_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aprksy/knitknot/extensions"
	"github.com/aprksy/knitknot/extensions/cti"
	"github.com/aprksy/knitknot/pkg/ports/types"
	"github.com/aprksy/knitknot/pkg/storage/inmem"
)

// bundleBody strips the trailing "# knitknot skipped ..." summary line, if // ext: cti-export
// present, so the remainder parses as pure bundle JSON. // ext: cti-export
func bundleBody(t *testing.T, out string) string { // ext: cti-export
	t.Helper()                        // ext: cti-export
	lines := strings.Split(out, "\n") // ext: cti-export
	var kept []string                 // ext: cti-export
	for _, l := range lines {         // ext: cti-export
		if strings.HasPrefix(l, "#") { // ext: cti-export
			continue // ext: cti-export
		} // ext: cti-export
		kept = append(kept, l) // ext: cti-export
	} // ext: cti-export
	return strings.Join(kept, "\n") // ext: cti-export
} // ext: cti-export

func importFixture(t *testing.T) *inmem.Storage { // ext: cti-export
	t.Helper()                                                                 // ext: cti-export
	data, err := os.ReadFile(filepath.Join("testdata", "minimal-bundle.json")) // ext: cti-export
	if err != nil {                                                            // ext: cti-export
		t.Fatalf("read fixture: %v", err) // ext: cti-export
	} // ext: cti-export
	s := inmem.New()                                                                                                                                         // ext: cti-export
	if err := cti.NewSTIXImporter().Import(context.Background(), extension.ImportContext{}, s, types.NewVerbRegistry(), bytes.NewReader(data)); err != nil { // ext: cti-export
		t.Fatalf("import: %v", err) // ext: cti-export
	} // ext: cti-export
	return s // ext: cti-export
} // ext: cti-export

func exportTo(t *testing.T, s *inmem.Storage) string { // ext: cti-export
	t.Helper()                                                                                                   // ext: cti-export
	var buf bytes.Buffer                                                                                         // ext: cti-export
	if err := cti.NewSTIXExporter().Export(context.Background(), s, types.NewVerbRegistry(), &buf); err != nil { // ext: cti-export
		t.Fatalf("export: %v", err) // ext: cti-export
	} // ext: cti-export
	return buf.String() // ext: cti-export
} // ext: cti-export

func TestSTIXExporterRoundTrip(t *testing.T) { // ext: cti-export
	out := exportTo(t, importFixture(t)) // ext: cti-export
	var doc struct {                     // ext: cti-export
		Type    string            `json:"type"`    // ext: cti-export
		ID      string            `json:"id"`      // ext: cti-export
		Objects []json.RawMessage `json:"objects"` // ext: cti-export
	} // ext: cti-export
	if err := json.Unmarshal([]byte(bundleBody(t, out)), &doc); err != nil { // ext: cti-export
		t.Fatalf("output is not valid JSON: %v\n%s", err, out) // ext: cti-export
	} // ext: cti-export
	if doc.Type != "bundle" { // ext: cti-export
		t.Fatalf("type = %q, want bundle", doc.Type) // ext: cti-export
	} // ext: cti-export
	if len(doc.Objects) != 9 { // ext: cti-export — 6 SDOs/custom + 3 relationships.
		t.Fatalf("objects = %d, want 9", len(doc.Objects)) // ext: cti-export
	} // ext: cti-export
	for _, want := range []string{ // ext: cti-export
		"indicator--a1a1a1a1-a1a1-a1a1-a1a1-a1a1a1a1a1a1",                 // ext: cti-export
		"malware--b2b2b2b2-b2b2-b2b2-b2b2-b2b2b2b2b2b2",                   // ext: cti-export
		"threat-actor--c3c3c3c3-c3c3-c3c3-c3c3-c3c3c3c3c3c3",              // ext: cti-export
		"identity--d4d4d4d4-d4d4-d4d4-d4d4-d4d4d4d4d4d4",                  // ext: cti-export
		"vulnerability--e5e5e5e5-e5e5-e5e5-e5e5-e5e5e5e5e5e5",             // ext: cti-export
		"x-custom-thing--f6f6f6f6-f6f6-f6f6-f6f6-f6f6f6f6f6f6",            // ext: cti-export
		"relationship--11111111-1111-1111-1111-111111111111",              // ext: cti-export
		"relationship--22222222-2222-2222-2222-222222222222",              // ext: cti-export
		"relationship--33333333-3333-3333-3333-333333333333",              // ext: cti-export
		`"source_ref": "indicator--a1a1a1a1-a1a1-a1a1-a1a1-a1a1a1a1a1a1"`, // ext: cti-export
		`"target_ref": "malware--b2b2b2b2-b2b2-b2b2-b2b2-b2b2b2b2b2b2"`,   // ext: cti-export
		`"relationship_type": "indicates"`,                                // ext: cti-export
		`"relationship_type": "uses"`,                                     // ext: cti-export
		`"relationship_type": "targets"`,                                  // ext: cti-export
	} { // ext: cti-export
		if !strings.Contains(out, want) { // ext: cti-export
			t.Fatalf("output missing %s", want) // ext: cti-export
		} // ext: cti-export
	} // ext: cti-export
	if strings.Contains(out, "source_feed") || strings.Contains(out, "imported_at") { // ext: cti-export
		t.Fatal("knitknot-internal props leaked into bundle") // ext: cti-export
	} // ext: cti-export
	if strings.Contains(out, "# knitknot skipped") { // ext: cti-export
		t.Fatal("skip summary present with nothing skipped") // ext: cti-export
	} // ext: cti-export
} // ext: cti-export

func TestSTIXExporterUnknownSkipped(t *testing.T) { // ext: cti-export
	s := importFixture(t)                 // ext: cti-export
	graphID, err := s.AddNode("foo", nil) // ext: cti-export
	if err != nil {                       // ext: cti-export
		t.Fatalf("add node: %v", err) // ext: cti-export
	} // ext: cti-export
	out := exportTo(t, s)                                      // ext: cti-export
	if strings.Contains(bundleBody(t, out), `"`+graphID+`"`) { // ext: cti-export
		t.Fatalf("non-STIX node %q leaked into bundle", graphID) // ext: cti-export
	} // ext: cti-export
	if !strings.Contains(out, "# knitknot skipped 1 nodes and 0 edges") { // ext: cti-export
		t.Fatalf("skip summary missing, got:\n%s", out) // ext: cti-export
	} // ext: cti-export
} // ext: cti-export

func TestSTIXExporterEmptyGraph(t *testing.T) { // ext: cti-export
	out := exportTo(t, inmem.New()) // ext: cti-export
	var doc struct {                // ext: cti-export
		Type    string            `json:"type"`    // ext: cti-export
		Objects []json.RawMessage `json:"objects"` // ext: cti-export
	} // ext: cti-export
	if err := json.Unmarshal([]byte(bundleBody(t, out)), &doc); err != nil { // ext: cti-export
		t.Fatalf("output is not valid JSON: %v\n%s", err, out) // ext: cti-export
	} // ext: cti-export
	if doc.Type != "bundle" { // ext: cti-export
		t.Fatalf("type = %q, want bundle", doc.Type) // ext: cti-export
	} // ext: cti-export
	if doc.Objects == nil || len(doc.Objects) != 0 { // ext: cti-export
		t.Fatalf("objects = %v, want empty array", doc.Objects) // ext: cti-export
	} // ext: cti-export
	if strings.Contains(out, "# knitknot skipped") { // ext: cti-export
		t.Fatal("skip summary present for empty graph") // ext: cti-export
	} // ext: cti-export
} // ext: cti-export
