// ext: cti-tactics — tactic materialization tests (ADR 0005, delivery stage 1).
package cti_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/aprksy/knitknot/extensions"
	"github.com/aprksy/knitknot/extensions/cti"
	"github.com/aprksy/knitknot/pkg/graph"
	"github.com/aprksy/knitknot/pkg/ports/types"
	"github.com/aprksy/knitknot/pkg/storage/inmem"
)

func readTacticsFixture(t *testing.T) []byte { // ext: cti-tactics
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "tactics-bundle.json"))
	if err != nil {
		t.Fatalf("read tactics fixture: %v", err)
	}
	return data
}

func importTactics(t *testing.T, s *inmem.Storage, data []byte, withTactics bool) { // ext: cti-tactics
	t.Helper()
	ic := extension.ImportContext{}
	if withTactics {
		ic = extension.ImportContext{Source: "attack-fixture", MaterializeTactics: true}
	}
	if err := cti.NewSTIXImporter().Import(context.Background(), ic, s, types.NewVerbRegistry(), bytes.NewReader(data)); err != nil {
		t.Fatalf("import: %v", err)
	}
}

// dropObjects returns bundle JSON with objects matching keep=false removed. // ext: cti-tactics
func dropObjects(t *testing.T, data []byte, drop func(obj map[string]any) bool) []byte { // ext: cti-tactics
	t.Helper()
	var bundle map[string]any
	if err := json.Unmarshal(data, &bundle); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	rawObjs, _ := bundle["objects"].([]any)
	kept := make([]any, 0, len(rawObjs))
	for _, o := range rawObjs {
		m, _ := o.(map[string]any)
		if m != nil && drop(m) {
			continue
		}
		kept = append(kept, o)
	}
	bundle["objects"] = kept
	out, err := json.Marshal(bundle)
	if err != nil {
		t.Fatalf("remarshal fixture: %v", err)
	}
	return out
}

func rankOf(t *testing.T, n *types.Node) int { // ext: cti-tactics
	t.Helper()
	switch v := n.Props[cti.PropRank].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		t.Fatalf("node %q rank = %v (%T), want a number", n.ID, n.Props[cti.PropRank], n.Props[cti.PropRank])
		return 0
	}
}

func tacticsByShort(t *testing.T, s *inmem.Storage) map[string]*types.Node { // ext: cti-tactics
	t.Helper()
	out := map[string]*types.Node{}
	for _, n := range s.GetNodesByLabel(cti.LabelTactic) {
		short, _ := n.Props[cti.PropShortname].(string)
		if short == "" {
			t.Fatalf("tactic node %q missing x_mitre_shortname", n.ID)
		}
		out[short] = n
	}
	return out
}

func hasTacticEdgesFrom(s *inmem.Storage, from string) int { // ext: cti-tactics
	n := 0
	for _, e := range s.GetEdgesFrom(from) {
		if e.Kind == cti.EdgeHasTactic {
			n++
		}
	}
	return n
}

func patternByName(t *testing.T, s *inmem.Storage, name string) *types.Node { // ext: cti-tactics
	t.Helper()
	for _, n := range s.GetNodesByLabel(cti.LabelAttackPattern) {
		if v, _ := n.Props["name"].(string); v == name {
			return n
		}
	}
	t.Fatalf("attack-pattern %q not found", name)
	return nil
}

func TestMaterializeTacticsRanksAndEdges(t *testing.T) { // ext: cti-tactics
	s := inmem.New()
	importTactics(t, s, readTacticsFixture(t), true)

	// Matrix tactic_refs order is execution, persistence, credential-access,
	// sitting at canonical fallback positions 4, 5, 9 — not 1, 2, 3.
	byShort := tacticsByShort(t, s)
	if len(byShort) != 3 {
		t.Fatalf("tactic nodes = %d, want 3", len(byShort))
	}
	for short, wantRank := range map[string]int{"execution": 4, "persistence": 5, "credential-access": 9} {
		n, ok := byShort[short]
		if !ok {
			t.Fatalf("missing tactic node %q", short)
		}
		if got := rankOf(t, n); got != wantRank {
			t.Fatalf("tactic %q rank = %d, want %d", short, got, wantRank)
		}
	}
	if v, _ := byShort["execution"].Props[cti.PropExternalID].(string); v != "TA0002" {
		t.Fatalf("execution external_id = %q, want TA0002", v)
	}
	if v, _ := byShort["credential-access"].Props["name"].(string); v != "Credential Access" {
		t.Fatalf("credential-access name = %q, want Credential Access", v)
	}
	for _, n := range byShort {
		if v, _ := n.Props[cti.SourceFeed].(string); v != "attack-fixture" {
			t.Fatalf("tactic %q source_feed = %q, want attack-fixture", n.ID, v)
		}
		if v, _ := n.Props[cti.ImportedAt].(string); v == "" {
			t.Fatalf("tactic %q missing imported_at", n.ID)
		}
	}

	single := patternByName(t, s, "Single Tactic Technique")
	if got := hasTacticEdgesFrom(s, single.ID); got != 1 {
		t.Fatalf("single-tactic pattern edges = %d, want 1", got)
	}
	multi := patternByName(t, s, "Multi Tactic Technique")
	if got := hasTacticEdgesFrom(s, multi.ID); got != 2 {
		t.Fatalf("multi-tactic pattern edges = %d, want 2 (not collapsed)", got)
	}
	if got := len(s.GetEdgesByKind(cti.EdgeHasTactic)); got != 3 {
		t.Fatalf("has-tactic edges = %d, want 3", got)
	}
}

func TestMaterializeTacticsFallbackNoMatrix(t *testing.T) { // ext: cti-tactics
	data := readTacticsFixture(t)
	// Drop the matrix AND one tactic object: ranks must come from the fixed
	// table and the missing tactic must be synthesized (marked, not fabricated).
	data = dropObjects(t, data, func(obj map[string]any) bool {
		typ, _ := obj["type"].(string)
		id, _ := obj["id"].(string)
		return typ == "x-mitre-matrix" ||
			id == "x-mitre-tactic--33333333-3333-3333-3333-333333333333"
	})
	s := inmem.New()
	importTactics(t, s, data, true)

	byShort := tacticsByShort(t, s)
	if len(byShort) != 3 {
		t.Fatalf("tactic nodes = %d, want 3 (2 imported + 1 synthesized)", len(byShort))
	}
	for short, wantRank := range map[string]int{"execution": 4, "persistence": 5, "credential-access": 9} {
		n, ok := byShort[short]
		if !ok {
			t.Fatalf("missing tactic node %q", short)
		}
		if got := rankOf(t, n); got != wantRank {
			t.Fatalf("tactic %q rank = %d, want %d (fallback table)", short, got, wantRank)
		}
	}
	synth := byShort["credential-access"]
	if v, _ := synth.Props[cti.PropSynthesized].(bool); !v {
		t.Fatal("synthesized tactic node missing synthesized:true")
	}
	if v, _ := synth.Props["name"].(string); v != "Credential Access" {
		t.Fatalf("synthesized name = %q, want Credential Access", v)
	}
	if v, _ := synth.Props[cti.StixID].(string); v != "" {
		t.Fatalf("synthesized stix_id = %q, want empty (never fabricated)", v)
	}
	if got := len(s.GetEdgesByKind(cti.EdgeHasTactic)); got != 3 {
		t.Fatalf("has-tactic edges = %d, want 3", got)
	}
}

func TestMaterializeTacticsNoTactics(t *testing.T) { // ext: cti-tactics
	bundle := `{
  "type": "bundle",
  "id": "bundle--88888888-8888-8888-8888-888888888888",
  "objects": [
    {"type": "attack-pattern", "spec_version": "2.1",
     "id": "attack-pattern--99999999-9999-9999-9999-999999999999",
     "created": "2024-01-01T00:00:00.000Z", "modified": "2024-01-01T00:00:00.000Z",
     "name": "Phaseless Technique"}
  ]
}`
	s := inmem.New()
	importTactics(t, s, []byte(bundle), true)
	if got := len(s.GetNodesByLabel(cti.LabelTactic)); got != 0 {
		t.Fatalf("tactic nodes = %d, want 0", got)
	}
	if got := len(s.GetEdgesByKind(cti.EdgeHasTactic)); got != 0 {
		t.Fatalf("has-tactic edges = %d, want 0", got)
	}
}

func TestMaterializeTacticsIdempotent(t *testing.T) { // ext: cti-tactics
	s := inmem.New()
	data := readTacticsFixture(t)
	importTactics(t, s, data, true)
	nodes, edges := len(s.GetAllNodes()), len(s.GetAllEdges())
	importTactics(t, s, data, true)
	if got := len(s.GetAllNodes()); got != nodes {
		t.Fatalf("nodes = %d after reimport, want %d", got, nodes)
	}
	if got := len(s.GetAllEdges()); got != edges {
		t.Fatalf("edges = %d after reimport, want %d", got, edges)
	}
}

func TestMaterializeTacticsQueryable(t *testing.T) { // ext: cti-tactics
	s := inmem.New()
	importTactics(t, s, readTacticsFixture(t), true)

	engine := graph.NewGraphEngine(s)
	reg := extension.NewRegistry()
	if err := cti.New().Register(reg); err != nil {
		t.Fatalf("register: %v", err)
	}
	for name, v := range reg.Verbs() {
		engine.RegisterVerb(name, v)
	}

	verb, ok := reg.Verbs()[cti.EdgeHasTactic]
	if !ok {
		t.Fatal("has-tactic verb not registered")
	}
	if verb.TargetLabel != cti.LabelTactic || verb.MatchOn != cti.PropShortname {
		t.Fatalf("has-tactic verb = %+v, want {x-mitre-tactic x_mitre_shortname}", verb)
	}

	res, err := engine.Find("attack-pattern").Has("has-tactic", "credential-access").Exec(context.Background())
	if err != nil {
		t.Fatalf("Has query: %v", err)
	}
	if res.Len() != 1 {
		t.Fatalf("Has('has-tactic','credential-access') rows = %d, want 1", res.Len())
	}
	if name, _ := res.Items()[0]["n"].Props["name"].(string); name != "Multi Tactic Technique" {
		t.Fatalf("Has match = %q, want Multi Tactic Technique", name)
	}

	followed, err := engine.Find("attack-pattern").Follow("has-tactic", graph.Out).Exec(context.Background())
	if err != nil {
		t.Fatalf("Follow query: %v", err)
	}
	if followed.Len() != 3 {
		t.Fatalf("Follow('has-tactic','out') rows = %d, want 3", followed.Len())
	}
	for _, row := range followed.Items() {
		if row["v0"].Label != cti.LabelTactic {
			t.Fatalf("Follow target label = %q, want x-mitre-tactic", row["v0"].Label)
		}
		if _, ok := row["v0"].Props[cti.PropShortname].(string); !ok {
			t.Fatalf("Follow target %q missing x_mitre_shortname", row["v0"].ID)
		}
	}
}

func TestDefaultImportNoMaterialization(t *testing.T) { // ext: cti-tactics
	s := inmem.New()
	importTactics(t, s, readTacticsFixture(t), false)

	// Tactic objects still import verbatim as plain nodes, but gain no rank
	// and no edges without the flag.
	imported := s.GetNodesByLabel(cti.LabelTactic)
	if len(imported) != 3 {
		t.Fatalf("tactic nodes = %d, want 3 (verbatim import)", len(imported))
	}
	for _, n := range imported {
		if _, ok := n.Props[cti.PropRank]; ok {
			t.Fatalf("tactic %q has rank without --tactics", n.ID)
		}
	}
	if got := len(s.GetEdgesByKind(cti.EdgeHasTactic)); got != 0 {
		t.Fatalf("has-tactic edges = %d without --tactics, want 0", got)
	}
	// No synthesized nodes sneak in either.
	for _, n := range s.GetAllNodes() {
		if v, _ := n.Props[cti.PropSynthesized].(bool); v {
			t.Fatalf("synthesized node %q without --tactics", n.ID)
		}
	}
}

func TestMaterializeTacticsSkipsForeignChains(t *testing.T) { // ext: cti-tactics
	bundle := `{
  "type": "bundle",
  "id": "bundle--aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
  "objects": [
    {"type": "attack-pattern", "spec_version": "2.1",
     "id": "attack-pattern--bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
     "created": "2024-01-01T00:00:00.000Z", "modified": "2024-01-01T00:00:00.000Z",
     "name": "Foreign Chain Technique",
     "kill_chain_phases": [
       {"kill_chain_name": "lockheed-martin-cyber-kill-chain", "phase_name": "reconnaissance"}
     ]}
  ]
}`
	s := inmem.New()
	importTactics(t, s, []byte(bundle), true)
	if got := len(s.GetNodesByLabel(cti.LabelTactic)); got != 0 {
		t.Fatalf("tactic nodes = %d, want 0 (foreign chain skipped)", got)
	}
	if got := len(s.GetEdgesByKind(cti.EdgeHasTactic)); got != 0 {
		t.Fatalf("has-tactic edges = %d, want 0", got)
	}
}
