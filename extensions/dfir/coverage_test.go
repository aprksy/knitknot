package dfir

import (
	"bytes"
	"context"
	"os"
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
			{Kind: "file-hash-sha256", Value: "ABCDEF", Context: "hash hit", Order: 0},
			{Kind: "ipv4", Value: "1.2.3.4", Order: 1},
			{Kind: "technique", Value: "Credential Dumping", Context: "analyst note", Order: 2},
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

// seedOrderedGraph imports testdata/ordered-bundle.json: an intrusion-set
// ("Ordered APT") over a campaign/malware chain with three ranked
// techniques (Command Shell=1, Credential Dumping=2, Remote Desktop=3) and
// one unranked technique (Unphased Tool). tactics=false skips --tactics, so
// no ranks or has-tactic edges exist.
func seedOrderedGraph(t *testing.T, tactics bool) *inmem.Storage {
	t.Helper()
	data, err := os.ReadFile("testdata/ordered-bundle.json")
	if err != nil {
		t.Fatalf("read ordered fixture: %v", err)
	}
	ic := extension.ImportContext{Source: "ordered-fixture"}
	if tactics {
		ic.MaterializeTactics = true
	}
	s := inmem.New()
	if err := cti.NewSTIXImporter().Import(context.Background(), ic, s, types.NewVerbRegistry(), bytes.NewReader(data)); err != nil {
		t.Fatalf("import: %v", err)
	}
	return s
}

func orderedTarget(t *testing.T, s *inmem.Storage) *types.Node {
	t.Helper()
	return mustResolve(t, s, "intrusion-set", "Ordered APT")
}

func orderedValue(t *testing.T, rep CoverageReport) (float64, int) {
	t.Helper()
	ap := dimByLabel(rep, "attack-pattern")
	if ap.OrderedCoverage == nil {
		t.Fatalf("ordered_coverage is nil (N=%d), want a value", ap.OrderedN)
	}
	return *ap.OrderedCoverage, ap.OrderedN
}

func TestTechniqueRank(t *testing.T) {
	s := inmem.New()
	ap, err := s.AddNode("attack-pattern", map[string]any{"name": "Multi"})
	if err != nil {
		t.Fatalf("AddNode: %v", err)
	}
	lo, err := s.AddNode("x-mitre-tactic", map[string]any{"x_mitre_shortname": "execution", "rank": 4})
	if err != nil {
		t.Fatalf("AddNode: %v", err)
	}
	hi, err := s.AddNode("x-mitre-tactic", map[string]any{"x_mitre_shortname": "persistence", "rank": float64(5)})
	if err != nil {
		t.Fatalf("AddNode: %v", err)
	}
	if err := s.AddEdge(ap, lo, "has-tactic", nil); err != nil {
		t.Fatalf("AddEdge: %v", err)
	}
	if err := s.AddEdge(ap, hi, "has-tactic", nil); err != nil {
		t.Fatalf("AddEdge: %v", err)
	}
	n, _ := s.GetNode(ap)
	if r, ok := techniqueRank(s, n); !ok || r != 4 {
		t.Errorf("multi-tactic rank = (%d, %v), want (4, true)", r, ok)
	}

	plain, err := s.AddNode("attack-pattern", map[string]any{"name": "Unphased"})
	if err != nil {
		t.Fatalf("AddNode: %v", err)
	}
	pn, _ := s.GetNode(plain)
	if _, ok := techniqueRank(s, pn); ok {
		t.Error("unlinked technique: ok = true, want false")
	}
}

func TestLISLength(t *testing.T) {
	for _, tc := range []struct {
		name string
		seq  []int
		want int
	}{
		{"empty", nil, 0},
		{"single", []int{2}, 1},
		{"in order", []int{1, 2, 3}, 3},
		{"reversed", []int{3, 2, 1}, 1},
		{"partial", []int{3, 1, 2}, 2},
		{"equal ranks", []int{1, 1, 2, 2}, 4}, // non-decreasing: ties are not violations
		{"equal ends", []int{2, 1, 1}, 2},
	} {
		if got := lisLength(tc.seq); got != tc.want {
			t.Errorf("%s: lisLength(%v) = %d, want %d", tc.name, tc.seq, got, tc.want)
		}
	}
}

func TestOrderedCoverageInOrder(t *testing.T) {
	s := seedOrderedGraph(t, true)
	rep, err := Coverage(s, orderedTarget(t, s), []CaseEntity{
		{Kind: EntityTechnique, Value: "Command Shell", Order: 1},
		{Kind: EntityTechnique, Value: "Credential Dumping", Order: 2},
		{Kind: EntityTechnique, Value: "Remote Desktop", Order: 3},
	}, 3)
	if err != nil {
		t.Fatalf("Coverage: %v", err)
	}
	got, n := orderedValue(t, rep)
	if got != 1.0 || n != 3 {
		t.Errorf("ordered = %v (N=%d), want 1.0 (N=3)", got, n)
	}
	ap := dimByLabel(rep, "attack-pattern")
	if names := refNames(ap.ActualOrder); !equalStrings(names, []string{"Command Shell", "Credential Dumping", "Remote Desktop"}) {
		t.Errorf("actual = %v", names)
	}
	if names := refNames(ap.ExpectedOrder); !equalStrings(names, []string{"Command Shell", "Credential Dumping", "Remote Desktop"}) {
		t.Errorf("expected = %v", names)
	}
}

func TestOrderedCoverageViolated(t *testing.T) {
	s := seedOrderedGraph(t, true)
	t.Run("reversed", func(t *testing.T) {
		rep, err := Coverage(s, orderedTarget(t, s), []CaseEntity{
			{Kind: EntityTechnique, Value: "Remote Desktop", Order: 1},
			{Kind: EntityTechnique, Value: "Credential Dumping", Order: 2},
			{Kind: EntityTechnique, Value: "Command Shell", Order: 3},
		}, 3)
		if err != nil {
			t.Fatalf("Coverage: %v", err)
		}
		got, n := orderedValue(t, rep)
		if want := float64(1) / float64(3); got != want || n != 3 {
			t.Errorf("ordered = %v (N=%d), want %v (N=3)", got, n, want)
		}
	})
	t.Run("partial", func(t *testing.T) {
		rep, err := Coverage(s, orderedTarget(t, s), []CaseEntity{
			{Kind: EntityTechnique, Value: "Remote Desktop", Order: 1},
			{Kind: EntityTechnique, Value: "Command Shell", Order: 2},
			{Kind: EntityTechnique, Value: "Credential Dumping", Order: 3},
		}, 3)
		if err != nil {
			t.Fatalf("Coverage: %v", err)
		}
		got, n := orderedValue(t, rep)
		if want := float64(2) / float64(3); got != want || n != 3 {
			t.Errorf("ordered = %v (N=%d), want %v (N=3)", got, n, want)
		}
		ap := dimByLabel(rep, "attack-pattern")
		if names := refNames(ap.ActualOrder); !equalStrings(names, []string{"Remote Desktop", "Command Shell", "Credential Dumping"}) {
			t.Errorf("actual = %v, want evidence order", names)
		}
		if names := refNames(ap.ExpectedOrder); !equalStrings(names, []string{"Command Shell", "Credential Dumping", "Remote Desktop"}) {
			t.Errorf("expected = %v, want rank order", names)
		}
	})
}

func TestOrderedCoverageUnrankedExcluded(t *testing.T) {
	s := seedOrderedGraph(t, true)
	rep, err := Coverage(s, orderedTarget(t, s), []CaseEntity{
		{Kind: EntityTechnique, Value: "Command Shell", Order: 1},
		{Kind: EntityTechnique, Value: "Unphased Tool", Order: 2},
		{Kind: EntityTechnique, Value: "Credential Dumping", Order: 3},
		{Kind: EntityTechnique, Value: "Remote Desktop", Order: 4},
	}, 3)
	if err != nil {
		t.Fatalf("Coverage: %v", err)
	}
	ap := dimByLabel(rep, "attack-pattern")
	if ap.Covered != 4 || ap.Total != 4 {
		t.Errorf("set coverage = %d/%d, want 4/4 (unranked still counts)", ap.Covered, ap.Total)
	}
	if ap.OrderedCoverage == nil || *ap.OrderedCoverage != 1.0 || ap.OrderedN != 3 {
		t.Errorf("ordered = %v (N=%d), want 1.0 (N=3, unranked excluded)", ap.OrderedCoverage, ap.OrderedN)
	}
	for _, r := range ap.ActualOrder {
		if r.Name == "Unphased Tool" {
			t.Errorf("actual order contains unranked %q", r.Name)
		}
	}
}

func TestOrderedCoverageOrderColumn(t *testing.T) {
	s := seedOrderedGraph(t, true)
	target := orderedTarget(t, s)
	t.Run("column beats row position", func(t *testing.T) {
		// Rows list Remote Desktop first, but the order column says it
		// came last: evidence order is Command, Credential, Remote.
		entities, err := ParseCaseEntities(strings.NewReader(
			"type,value,order\ntechnique,Remote Desktop,3\ntechnique,Command Shell,1\ntechnique,Credential Dumping,2\n"))
		if err != nil {
			t.Fatalf("ParseCaseEntities: %v", err)
		}
		rep, err := Coverage(s, target, entities, 3)
		if err != nil {
			t.Fatalf("Coverage: %v", err)
		}
		if got, n := orderedValue(t, rep); got != 1.0 || n != 3 {
			t.Errorf("ordered = %v (N=%d), want 1.0 (N=3)", got, n)
		}
	})
	t.Run("row-order fallback", func(t *testing.T) {
		// No order column: row position is the sequence, so this is
		// fully reversed.
		entities, err := ParseCaseEntities(strings.NewReader(
			"type,value\ntechnique,Remote Desktop\ntechnique,Credential Dumping\ntechnique,Command Shell\n"))
		if err != nil {
			t.Fatalf("ParseCaseEntities: %v", err)
		}
		rep, err := Coverage(s, target, entities, 3)
		if err != nil {
			t.Fatalf("Coverage: %v", err)
		}
		if got, n := orderedValue(t, rep); got != float64(1)/float64(3) || n != 3 {
			t.Errorf("ordered = %v (N=%d), want 1/3 (N=3)", got, n)
		}
	})
}

func TestOrderedCoverageNA(t *testing.T) {
	t.Run("no tactics materialized", func(t *testing.T) {
		s := seedOrderedGraph(t, false)
		rep, err := Coverage(s, orderedTarget(t, s), []CaseEntity{
			{Kind: EntityTechnique, Value: "Command Shell", Order: 1},
			{Kind: EntityTechnique, Value: "Remote Desktop", Order: 2},
		}, 3)
		if err != nil {
			t.Fatalf("Coverage: %v", err)
		}
		ap := dimByLabel(rep, "attack-pattern")
		if ap.OrderedCoverage != nil || ap.OrderedN != 0 {
			t.Errorf("ordered = %v (N=%d), want nil (N=0)", ap.OrderedCoverage, ap.OrderedN)
		}
		if ap.Covered != 2 {
			t.Errorf("set coverage = %d, want 2 (ordering n/a never removes set coverage)", ap.Covered)
		}
	})
	t.Run("no technique evidence", func(t *testing.T) {
		s := seedOrderedGraph(t, true)
		rep, err := Coverage(s, orderedTarget(t, s), []CaseEntity{
			{Kind: "domain", Value: "ordered-c2.example.com"},
		}, 3)
		if err != nil {
			t.Fatalf("Coverage: %v", err)
		}
		ap := dimByLabel(rep, "attack-pattern")
		if ap.OrderedCoverage != nil || ap.OrderedN != 0 {
			t.Errorf("ordered = %v (N=%d), want nil (N=0)", ap.OrderedCoverage, ap.OrderedN)
		}
	})
	t.Run("indicator dimension never ordered", func(t *testing.T) {
		s := seedOrderedGraph(t, true)
		rep, err := Coverage(s, orderedTarget(t, s), []CaseEntity{
			{Kind: "domain", Value: "ordered-c2.example.com"},
			{Kind: EntityTechnique, Value: "Command Shell", Order: 1},
		}, 3)
		if err != nil {
			t.Fatalf("Coverage: %v", err)
		}
		if ind := dimByLabel(rep, "indicator"); ind.OrderedCoverage != nil || ind.OrderedN != 0 {
			t.Errorf("indicator ordered = %v (N=%d), want nil (N=0)", ind.OrderedCoverage, ind.OrderedN)
		}
	})
}

func TestParseCaseEntitiesOrder(t *testing.T) {
	t.Run("explicit column", func(t *testing.T) {
		got, err := ParseCaseEntities(strings.NewReader("type,value,order\ntechnique,B,30\ntechnique,A,7\n"))
		if err != nil {
			t.Fatalf("ParseCaseEntities: %v", err)
		}
		if len(got) != 2 || got[0].Order != 30 || got[1].Order != 7 {
			t.Errorf("got %+v, want orders [30 7]", got)
		}
	})

	t.Run("absent column falls back to row index", func(t *testing.T) {
		got, err := ParseCaseEntities(strings.NewReader("type,value\ndomain,a.example\ntechnique,B\n"))
		if err != nil {
			t.Fatalf("ParseCaseEntities: %v", err)
		}
		if len(got) != 2 || got[0].Order != 0 || got[1].Order != 1 {
			t.Errorf("got %+v, want orders [0 1]", got)
		}
	})

	t.Run("empty cell falls back to row index", func(t *testing.T) {
		got, err := ParseCaseEntities(strings.NewReader("type,value,order\ntechnique,A,\ntechnique,B,5\n"))
		if err != nil {
			t.Fatalf("ParseCaseEntities: %v", err)
		}
		if len(got) != 2 || got[0].Order != 0 || got[1].Order != 5 {
			t.Errorf("got %+v, want orders [0 5]", got)
		}
	})

	t.Run("bad order errors with row", func(t *testing.T) {
		_, err := ParseCaseEntities(strings.NewReader("type,value,order\ntechnique,A,first\n"))
		if err == nil || !strings.Contains(err.Error(), "row 2") || !strings.Contains(err.Error(), "first") {
			t.Errorf("err = %v, want row-and-value naming error", err)
		}
	})

	t.Run("legacy files unaffected", func(t *testing.T) {
		got, err := ParseCaseEntities(strings.NewReader("type,value,context\ndomain,example.com,seen\n"))
		if err != nil {
			t.Fatalf("ParseCaseEntities: %v", err)
		}
		if len(got) != 1 || got[0].Order != 0 || got[0].Context != "seen" {
			t.Errorf("got %+v, want one row with order 0", got)
		}
		obs, err := ParseCaseCSV(strings.NewReader("type,value,order\ndomain,example.com,9\n"))
		if err != nil {
			t.Fatalf("ParseCaseCSV: %v", err)
		}
		if len(obs) != 1 || obs[0].Type != "domain" {
			t.Errorf("ParseCaseCSV got %+v, want the domain row (order ignored)", obs)
		}
	})
}
