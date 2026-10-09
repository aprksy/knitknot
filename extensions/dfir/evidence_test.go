package dfir

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/aprksy/knitknot/extensions"
	"github.com/aprksy/knitknot/pkg/ports/types"
	"github.com/aprksy/knitknot/pkg/storage/inmem"
)

func evidenceNodes(s *inmem.Storage) int {
	return len(s.GetNodesByLabel(LabelObservable))
}

func TestImportEvidenceCreates(t *testing.T) {
	s := inmem.New()
	stats, err := ImportEvidence(s, []CaseEntity{
		{Kind: "domain", Value: "Tutorial-C2.EXAMPLE.com.", Order: 1},
		{Kind: EntityTechnique, Value: "Credential Dumping", Order: 2},
	}, "window-1")
	if err != nil {
		t.Fatalf("ImportEvidence: %v", err)
	}
	if stats != (EvidenceStats{Added: 2}) {
		t.Errorf("stats = %+v, want {Added 2}", stats)
	}
	if got := evidenceNodes(s); got != 2 {
		t.Fatalf("observable nodes = %d, want 2", got)
	}

	// Observable props follow the workspace layer: canonical type/value/name.
	dom, err := ResolveTarget(s, LabelObservable, "tutorial-c2.example.com")
	if err != nil {
		t.Fatalf("domain node: %v", err)
	}
	for k, want := range map[string]any{"type": "domain", "value": "tutorial-c2.example.com", "order": 1, PropSourceFeed: "window-1"} {
		if dom.Props[k] != want {
			t.Errorf("domain prop %q = %v, want %v", k, dom.Props[k], want)
		}
	}

	// Technique nodes keep the display value, keyed case-insensitively.
	tech, err := ResolveTarget(s, LabelObservable, "credential dumping")
	if err != nil {
		t.Fatalf("technique node: %v", err)
	}
	if v, _ := tech.Props["value"].(string); v != "Credential Dumping" {
		t.Errorf("technique value = %q, want original case", v)
	}
	if v, _ := tech.Props["type"].(string); v != "technique" {
		t.Errorf("technique type = %q", v)
	}
}

func TestImportEvidenceMerges(t *testing.T) {
	s := inmem.New()
	first := []CaseEntity{
		{Kind: "domain", Value: "example.com", Order: 5},
		{Kind: EntityTechnique, Value: "Credential Dumping", Order: 9},
	}
	if _, err := ImportEvidence(s, first, "window-1"); err != nil {
		t.Fatalf("ImportEvidence: %v", err)
	}

	// Same identities, different case/order/window: merge, keep minima.
	stats, err := ImportEvidence(s, []CaseEntity{
		{Kind: "domain", Value: "EXAMPLE.com", Order: 2},
		{Kind: EntityTechnique, Value: "CREDENTIAL DUMPING", Order: 3},
	}, "window-2")
	if err != nil {
		t.Fatalf("ImportEvidence: %v", err)
	}
	if stats != (EvidenceStats{Merged: 2}) {
		t.Errorf("stats = %+v, want {Merged 2}", stats)
	}
	if got := evidenceNodes(s); got != 2 {
		t.Errorf("observable nodes = %d, want 2 (no duplicates)", got)
	}

	dom, _ := ResolveTarget(s, LabelObservable, "example.com")
	if o, _ := asRank(dom.Props["order"]); o != 2 {
		t.Errorf("domain order = %d, want min 2", o)
	}
	if sf, _ := dom.Props[PropSourceFeed].(string); sf != "window-1" {
		t.Errorf("domain source_feed = %q, want first-seen window-1", sf)
	}
	tech, _ := ResolveTarget(s, LabelObservable, "credential dumping")
	if o, _ := asRank(tech.Props["order"]); o != 3 {
		t.Errorf("technique order = %d, want min 3", o)
	}
	if v, _ := tech.Props["value"].(string); v != "Credential Dumping" {
		t.Errorf("technique value = %q, want first-seen display", v)
	}
	if got := len(dom.History); got != 2 {
		t.Errorf("domain history snapshots = %d, want 2 (one per observation)", got)
	}
}

func TestImportEvidenceIdempotent(t *testing.T) {
	s := inmem.New()
	entities := []CaseEntity{
		{Kind: "domain", Value: "example.com", Order: 1},
		{Kind: EntityTechnique, Value: "Credential Dumping", Order: 2},
	}
	if _, err := ImportEvidence(s, entities, "window-1"); err != nil {
		t.Fatalf("ImportEvidence: %v", err)
	}
	stats, err := ImportEvidence(s, entities, "window-1")
	if err != nil {
		t.Fatalf("re-import: %v", err)
	}
	if stats.Added != 0 {
		t.Errorf("re-import added = %d, want 0", stats.Added)
	}
	if got := evidenceNodes(s); got != 2 {
		t.Errorf("observable nodes = %d, want 2", got)
	}
}

func TestImportEvidenceAccumulatesWindows(t *testing.T) {
	s := inmem.New()
	if _, err := ImportEvidence(s, []CaseEntity{{Kind: "domain", Value: "a.example", Order: 1}}, "w1"); err != nil {
		t.Fatalf("w1: %v", err)
	}
	stats, err := ImportEvidence(s, []CaseEntity{{Kind: "domain", Value: "b.example", Order: 2}}, "w2")
	if err != nil {
		t.Fatalf("w2: %v", err)
	}
	if stats != (EvidenceStats{Added: 1}) {
		t.Errorf("w2 stats = %+v, want {Added 1}", stats)
	}
	if got := len(EvidenceFromGraph(s, "")); got != 2 {
		t.Errorf("all-windows entities = %d, want 2", got)
	}
	if got := len(EvidenceFromGraph(s, "w1")); got != 1 {
		t.Errorf("w1 entities = %d, want 1", got)
	}
}

func TestEvidenceFromGraphRoundTrip(t *testing.T) {
	s := seedCorrelateGraph(t)
	entities := []CaseEntity{
		{Kind: "domain", Value: "tutorial-c2.example.com", Order: 1},
		{Kind: EntityTechnique, Value: "Credential Dumping", Order: 2},
	}
	if _, err := ImportEvidence(s, entities, "w1"); err != nil {
		t.Fatalf("ImportEvidence: %v", err)
	}
	target, err := ResolveTarget(s, "intrusion-set", "Tutorial APT")
	if err != nil {
		t.Fatalf("ResolveTarget: %v", err)
	}
	repCSV, err := Coverage(s, target, entities, 3)
	if err != nil {
		t.Fatalf("Coverage csv: %v", err)
	}
	repGraph, err := Coverage(s, target, EvidenceFromGraph(s, ""), 3)
	if err != nil {
		t.Fatalf("Coverage graph: %v", err)
	}
	if !reflect.DeepEqual(repCSV, repGraph) {
		t.Errorf("from-graph report differs from CSV report:\ncsv:  %+v\ngraph:%+v", repCSV, repGraph)
	}
}

func TestEvidenceImporterEndToEnd(t *testing.T) {
	s := seedCorrelateGraph(t)
	im := NewEvidenceImporter()
	if err := im.Import(context.Background(), extension.ImportContext{Source: "window-1"}, s, types.NewVerbRegistry(), strings.NewReader(
		"type,value,order\ndomain,tutorial-c2.example.com,1\ntechnique,Credential Dumping,2\n")); err != nil {
		t.Fatalf("Import: %v", err)
	}
	if got := evidenceNodes(s); got != 2 {
		t.Errorf("observable nodes = %d, want 2", got)
	}
	got := EvidenceFromGraph(s, "window-1")
	if len(got) != 2 || got[0].Value != "tutorial-c2.example.com" || got[1].Value != "Credential Dumping" {
		t.Errorf("window-1 entities = %+v, want domain then technique in order", got)
	}
}
