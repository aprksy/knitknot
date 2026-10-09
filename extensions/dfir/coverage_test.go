package dfir

import (
	"context"
	"strings"
	"testing"

	"github.com/aprksy/knitknot/extensions"
	"github.com/aprksy/knitknot/extensions/cti"
	"github.com/aprksy/knitknot/pkg/ports/types"
	"github.com/aprksy/knitknot/pkg/storage/inmem"
)

// coverageBundle mirrors docs/cti/samples/th-sample.json: two indicators
// and two attack-patterns hung off one malware/campaign/actor chain.
const coverageBundle = `{
  "type": "bundle",
  "id": "bundle--aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
  "spec_version": "2.1",
  "objects": [
    {"type": "indicator", "spec_version": "2.1",
     "id": "indicator--11111111-1111-1111-1111-111111111111",
     "created": "2024-01-01T00:00:00.000Z", "modified": "2024-01-01T00:00:00.000Z",
     "name": "Tutorial C2", "pattern": "[domain-name:value = 'tutorial-c2.example.com']",
     "pattern_type": "stix", "valid_from": "2024-01-01T00:00:00.000Z"},
    {"type": "indicator", "spec_version": "2.1",
     "id": "indicator--22222222-2222-2222-2222-222222222222",
     "created": "2024-01-01T00:00:00.000Z", "modified": "2024-01-01T00:00:00.000Z",
     "name": "Tutorial Hash", "pattern": "[file:hashes.'SHA-256' = 'a1b2c3d4e5f60718293a4b5c6d7e8f90a1b2c3d4e5f60718293a4b5c6d7e8f90']",
     "pattern_type": "stix", "valid_from": "2024-01-01T00:00:00.000Z"},
    {"type": "malware", "spec_version": "2.1",
     "id": "malware--33333333-3333-3333-3333-333333333333",
     "created": "2024-01-01T00:00:00.000Z", "modified": "2024-01-01T00:00:00.000Z",
     "name": "TutorialBanker", "malware_types": ["banking-trojan"], "is_family": false},
    {"type": "attack-pattern", "spec_version": "2.1",
     "id": "attack-pattern--44444444-4444-4444-4444-444444444444",
     "created": "2024-01-01T00:00:00.000Z", "modified": "2024-01-01T00:00:00.000Z",
     "name": "Credential Dumping"},
    {"type": "attack-pattern", "spec_version": "2.1",
     "id": "attack-pattern--aaaaaaa2-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
     "created": "2024-01-01T00:00:00.000Z", "modified": "2024-01-01T00:00:00.000Z",
     "name": "Lateral Movement"},
    {"type": "campaign", "spec_version": "2.1",
     "id": "campaign--55555555-5555-5555-5555-555555555555",
     "created": "2024-01-01T00:00:00.000Z", "modified": "2024-01-01T00:00:00.000Z",
     "name": "Tutorial Campaign X"},
    {"type": "intrusion-set", "spec_version": "2.1",
     "id": "intrusion-set--66666666-6666-6666-6666-666666666666",
     "created": "2024-01-01T00:00:00.000Z", "modified": "2024-01-01T00:00:00.000Z",
     "name": "Tutorial APT"},
    {"type": "relationship", "spec_version": "2.1",
     "id": "relationship--77777777-7777-7777-7777-777777777777",
     "created": "2024-01-02T00:00:00.000Z", "modified": "2024-01-02T00:00:00.000Z",
     "relationship_type": "indicates",
     "source_ref": "indicator--11111111-1111-1111-1111-111111111111",
     "target_ref": "malware--33333333-3333-3333-3333-333333333333"},
    {"type": "relationship", "spec_version": "2.1",
     "id": "relationship--88888888-8888-8888-8888-888888888888",
     "created": "2024-01-02T00:00:00.000Z", "modified": "2024-01-02T00:00:00.000Z",
     "relationship_type": "indicates",
     "source_ref": "indicator--22222222-2222-2222-2222-222222222222",
     "target_ref": "malware--33333333-3333-3333-3333-333333333333"},
    {"type": "relationship", "spec_version": "2.1",
     "id": "relationship--99999999-9999-9999-9999-999999999999",
     "created": "2024-01-02T00:00:00.000Z", "modified": "2024-01-02T00:00:00.000Z",
     "relationship_type": "uses",
     "source_ref": "malware--33333333-3333-3333-3333-333333333333",
     "target_ref": "attack-pattern--44444444-4444-4444-4444-444444444444"},
    {"type": "relationship", "spec_version": "2.1",
     "id": "relationship--ccccccc2-cccc-cccc-cccc-cccccccccccc",
     "created": "2024-01-02T00:00:00.000Z", "modified": "2024-01-02T00:00:00.000Z",
     "relationship_type": "uses",
     "source_ref": "malware--33333333-3333-3333-3333-333333333333",
     "target_ref": "attack-pattern--aaaaaaa2-aaaa-aaaa-aaaa-aaaaaaaaaaaa"},
    {"type": "relationship", "spec_version": "2.1",
     "id": "relationship--aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
     "created": "2024-01-02T00:00:00.000Z", "modified": "2024-01-02T00:00:00.000Z",
     "relationship_type": "uses",
     "source_ref": "campaign--55555555-5555-5555-5555-555555555555",
     "target_ref": "malware--33333333-3333-3333-3333-333333333333"},
    {"type": "relationship", "spec_version": "2.1",
     "id": "relationship--bbbbbbbb-cccc-dddd-eeee-ffffffffffff",
     "created": "2024-01-02T00:00:00.000Z", "modified": "2024-01-02T00:00:00.000Z",
     "relationship_type": "attributed-to",
     "source_ref": "campaign--55555555-5555-5555-5555-555555555555",
     "target_ref": "intrusion-set--66666666-6666-6666-6666-666666666666"}
   ]
}`

func seedCoverageGraph(t *testing.T) *inmem.Storage {
	t.Helper()
	s := inmem.New()
	if err := cti.NewSTIXImporter().Import(context.Background(), extension.ImportContext{}, s, types.NewVerbRegistry(), strings.NewReader(coverageBundle)); err != nil {
		t.Fatalf("import: %v", err)
	}
	return s
}

func mustResolve(t *testing.T, s *inmem.Storage, label, name string) *types.Node {
	t.Helper()
	n, err := ResolveTarget(s, label, name)
	if err != nil {
		t.Fatalf("ResolveTarget(%q, %q): %v", label, name, err)
	}
	return n
}

func patternNames(p map[string][]*types.Node, label string) []string {
	var out []string
	for _, n := range p[label] {
		name, _ := n.Props["name"].(string)
		out = append(out, name)
	}
	return out
}

func TestPatternOfBothDirections(t *testing.T) {
	s := seedCoverageGraph(t)

	// From the actor: reverse traversal must reach indicators and techniques.
	actor := mustResolve(t, s, "intrusion-set", "Tutorial APT")
	pat := PatternOf(s, actor, 3)
	if got := patternNames(pat, "indicator"); !equalStrings(got, []string{"Tutorial C2", "Tutorial Hash"}) {
		t.Errorf("actor pattern indicators = %v", got)
	}
	if got := patternNames(pat, "attack-pattern"); !equalStrings(got, []string{"Credential Dumping", "Lateral Movement"}) {
		t.Errorf("actor pattern attack-patterns = %v", got)
	}
	for _, n := range pat["intrusion-set"] {
		if name, _ := n.Props["name"].(string); name == "Tutorial APT" {
			t.Errorf("pattern contains the target itself")
		}
	}

	// From an indicator: forward traversal must reach the actor and the
	// techniques (via the malware).
	ind := mustResolve(t, s, "indicator", "Tutorial C2")
	pat = PatternOf(s, ind, 3)
	if got := patternNames(pat, "intrusion-set"); !equalStrings(got, []string{"Tutorial APT"}) {
		t.Errorf("indicator pattern actors = %v", got)
	}
	if got := patternNames(pat, "attack-pattern"); !equalStrings(got, []string{"Credential Dumping", "Lateral Movement"}) {
		t.Errorf("indicator pattern attack-patterns = %v", got)
	}
}

func TestDirectEvidenceMapping(t *testing.T) {
	s := seedCoverageGraph(t)
	entities := []CaseEntity{
		{Kind: "domain", Value: "tutorial-c2.example.com"},
		{Kind: EntityTechnique, Value: "credential dumping"}, // case-insensitive
		{Kind: "ipv4", Value: "203.0.113.9"},                 // matches nothing
		{Kind: EntityTechnique, Value: "No Such Technique"},
	}
	got := DirectEvidence(s, entities)
	if names := patternNames(got, "indicator"); !equalStrings(names, []string{"Tutorial C2"}) {
		t.Errorf("direct indicators = %v", names)
	}
	if names := patternNames(got, "attack-pattern"); !equalStrings(names, []string{"Credential Dumping"}) {
		t.Errorf("direct attack-patterns = %v", names)
	}
	if len(got["malware"]) != 0 || len(got["campaign"]) != 0 {
		t.Errorf("reachable-but-not-direct nodes must never be direct evidence: %v", got)
	}
}

func dimByLabel(rep CoverageReport, label string) DimensionCoverage {
	for _, d := range rep.Dimensions {
		if d.Label == label {
			return d
		}
	}
	return DimensionCoverage{}
}

func TestCoverageFractions(t *testing.T) {
	s := seedCoverageGraph(t)
	actor := mustResolve(t, s, "intrusion-set", "Tutorial APT")
	rep, err := Coverage(s, actor, []CaseEntity{
		{Kind: "domain", Value: "tutorial-c2.example.com"},
		{Kind: EntityTechnique, Value: "Credential Dumping"},
	}, 3)
	if err != nil {
		t.Fatalf("Coverage: %v", err)
	}
	ind := dimByLabel(rep, "indicator")
	if ind.Covered != 1 || ind.Total != 2 {
		t.Errorf("indicator = %d/%d, want 1/2", ind.Covered, ind.Total)
	}
	if got := refNames(ind.Matched); !equalStrings(got, []string{"Tutorial C2"}) {
		t.Errorf("indicator matched = %v", got)
	}
	if got := refNames(ind.Unmatched); !equalStrings(got, []string{"Tutorial Hash"}) {
		t.Errorf("indicator unmatched = %v", got)
	}
	ap := dimByLabel(rep, "attack-pattern")
	if ap.Covered != 1 || ap.Total != 2 {
		t.Errorf("attack-pattern = %d/%d, want 1/2", ap.Covered, ap.Total)
	}
	if got := refNames(ap.Matched); !equalStrings(got, []string{"Credential Dumping"}) {
		t.Errorf("attack-pattern matched = %v", got)
	}
	if got := refNames(ap.Unmatched); !equalStrings(got, []string{"Lateral Movement"}) {
		t.Errorf("attack-pattern unmatched = %v", got)
	}
	if len(rep.Dimensions) != 2 || rep.Dimensions[0].Label != "indicator" || rep.Dimensions[1].Label != "attack-pattern" {
		t.Errorf("dimensions must be [indicator attack-pattern] in stable order, got %v", rep.Dimensions)
	}
}

func TestCoverageSingleMatchDoesNotInflate(t *testing.T) {
	s := seedCoverageGraph(t)
	actor := mustResolve(t, s, "intrusion-set", "Tutorial APT")
	// One IOC reaches the whole chain — but only the indicator dimension counts it.
	rep, err := Coverage(s, actor, []CaseEntity{
		{Kind: "domain", Value: "tutorial-c2.example.com"},
	}, 3)
	if err != nil {
		t.Fatalf("Coverage: %v", err)
	}
	if ind := dimByLabel(rep, "indicator"); ind.Covered != 1 || ind.Total != 2 {
		t.Errorf("indicator = %d/%d, want 1/2", ind.Covered, ind.Total)
	}
	if ap := dimByLabel(rep, "attack-pattern"); ap.Covered != 0 || ap.Total != 2 {
		t.Errorf("attack-pattern = %d/%d, want 0/2 (no technique evidence)", ap.Covered, ap.Total)
	}
}

func TestCoverageEmptyDimensionIsNA(t *testing.T) {
	s := seedCoverageGraph(t)
	// The hash indicator's pattern neighbourhood has no attack-patterns
	// within depth 1 — Total 0 must stay 0/0, never 100%.
	ind := mustResolve(t, s, "indicator", "Tutorial Hash")
	rep, err := Coverage(s, ind, []CaseEntity{
		{Kind: "domain", Value: "tutorial-c2.example.com"},
	}, 1)
	if err != nil {
		t.Fatalf("Coverage: %v", err)
	}
	ap := dimByLabel(rep, "attack-pattern")
	if ap.Covered != 0 || ap.Total != 0 {
		t.Errorf("attack-pattern = %d/%d, want 0/0 (n/a)", ap.Covered, ap.Total)
	}
	if len(ap.Matched) != 0 {
		t.Errorf("empty dimension must match nothing, got %v", refNames(ap.Matched))
	}
}

func TestCoverageInferredExcludesDirect(t *testing.T) {
	s := seedCoverageGraph(t)
	actor := mustResolve(t, s, "intrusion-set", "Tutorial APT")
	rep, err := Coverage(s, actor, []CaseEntity{
		{Kind: "domain", Value: "tutorial-c2.example.com"},
		{Kind: EntityTechnique, Value: "Credential Dumping"},
	}, 3)
	if err != nil {
		t.Fatalf("Coverage: %v", err)
	}
	if len(rep.Inferred) == 0 {
		t.Fatalf("inferred context is empty, want malware/campaign/etc.")
	}
	covered := map[string]bool{}
	for _, d := range rep.Dimensions {
		for _, r := range d.Matched {
			covered[r.ID] = true
		}
	}
	for _, r := range rep.Inferred {
		if covered[r.ID] {
			t.Errorf("inferred contains directly-covered node %q", r.Name)
		}
		if r.ID == rep.Target.ID {
			t.Errorf("inferred contains the target itself")
		}
	}
	var labels []string
	for _, r := range rep.Inferred {
		labels = append(labels, r.Label+":"+r.Name)
	}
	t.Logf("inferred: %v", labels)
	found := false
	for _, r := range rep.Inferred {
		if r.Label == "malware" || r.Label == "campaign" {
			found = true
		}
	}
	if !found {
		t.Errorf("inferred %v lacks adversary context (malware/campaign)", labels)
	}
}

func TestResolveTargetErrors(t *testing.T) {
	s := seedCoverageGraph(t)
	if _, err := ResolveTarget(s, "intrusion-set", "Nobody"); err == nil {
		t.Error("missing target: err = nil")
	}
	id1, err := s.AddNode("campaign", map[string]any{"name": "Dup"})
	if err != nil {
		t.Fatalf("AddNode: %v", err)
	}
	if _, err := s.AddNode("campaign", map[string]any{"name": "Dup"}); err != nil {
		t.Fatalf("AddNode: %v", err)
	}
	_ = id1
	if _, err := ResolveTarget(s, "campaign", "Dup"); err == nil {
		t.Error("ambiguous target: err = nil")
	} else if !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("ambiguous target err = %v, want 'ambiguous'", err)
	}
}

func TestParseCaseEntities(t *testing.T) {
	t.Run("aliases and technique", func(t *testing.T) {
		in := "type,value,context\nSHA-256,ABCDEF,hash hit\nip,1.2.3.4\ntechnique,Credential Dumping,analyst note\n"
		got, err := ParseCaseEntities(strings.NewReader(in))
		if err != nil {
			t.Fatalf("ParseCaseEntities: %v", err)
		}
		want := []CaseEntity{
			{Kind: "file-hash-sha256", Value: "ABCDEF", Context: "hash hit"},
			{Kind: "ipv4", Value: "1.2.3.4"},
			{Kind: "technique", Value: "Credential Dumping", Context: "analyst note"},
		}
		if len(got) != len(want) {
			t.Fatalf("got %v, want %v", got, want)
		}
		for i := range got {
			if got[i] != want[i] {
				t.Errorf("row %d = %+v, want %+v", i, got[i], want[i])
			}
		}
	})

	t.Run("technique case-insensitive", func(t *testing.T) {
		got, err := ParseCaseEntities(strings.NewReader("type,value\nTECHNIQUE,Credential Dumping\n"))
		if err != nil {
			t.Fatalf("ParseCaseEntities: %v", err)
		}
		if len(got) != 1 || got[0].Kind != "technique" {
			t.Errorf("got %+v, want one technique entity", got)
		}
	})

	t.Run("unknown type errors", func(t *testing.T) {
		_, err := ParseCaseEntities(strings.NewReader("type,value\nemail,a@b.test\n"))
		if err == nil || !strings.Contains(err.Error(), "row 2") || !strings.Contains(err.Error(), "email") {
			t.Errorf("err = %v, want row-and-type naming error", err)
		}
	})

	t.Run("missing value errors", func(t *testing.T) {
		_, err := ParseCaseEntities(strings.NewReader("type,value\ntechnique,\n"))
		if err == nil || !strings.Contains(err.Error(), "row 2") {
			t.Errorf("err = %v, want row 2 error", err)
		}
	})

	t.Run("legacy csv skips techniques", func(t *testing.T) {
		got, err := ParseCaseCSV(strings.NewReader("type,value\ndomain,example.com\ntechnique,Credential Dumping\n"))
		if err != nil {
			t.Fatalf("ParseCaseCSV: %v", err)
		}
		if len(got) != 1 || got[0].Type != "domain" {
			t.Errorf("got %+v, want only the domain row", got)
		}
	})
}
