package dfir

import (
	"context"
	"reflect"
	"testing"

	extension "github.com/aprksy/knitknot/extensions"
	"github.com/aprksy/knitknot/extensions/cti"
	"github.com/aprksy/knitknot/pkg/graph"
	"github.com/aprksy/knitknot/pkg/storage/inmem"
)

func labelCounts(s *inmem.Storage) map[string]int {
	m := map[string]int{}
	for _, n := range s.GetAllNodes() {
		m[n.Label]++
	}
	return m
}

func kindCounts(s *inmem.Storage) map[string]int {
	m := map[string]int{}
	for _, e := range s.GetAllEdges() {
		m[e.Kind]++
	}
	return m
}

func TestBuildWorkspaceProjection(t *testing.T) {
	src := seedCorrelateGraph(t)
	wantNodes, wantEdges := len(src.GetAllNodes()), len(src.GetAllEdges())
	// Domain only: the hash indicator must stay out of the projection.
	caseObs := []CaseObservable{
		{Observable: Observable{Type: "domain", Value: "tutorial-c2.example.com"}, Context: "proxy log"},
	}
	dst := inmem.New()
	stats, err := BuildWorkspace(src, dst, caseObs, WorkspaceOptions{CaseSource: "case"})
	if err != nil {
		t.Fatalf("BuildWorkspace: %v", err)
	}

	if got := (WorkspaceStats{Observables: 1, Indicators: 1, Malware: 1, Campaigns: 1, Actors: 1, TTPs: 1, Edges: 5}); stats != got {
		t.Errorf("stats = %+v, want %+v", stats, got)
	}

	if got, want := labelCounts(dst), map[string]int{
		cti.LabelObservable: 1, cti.LabelIndicator: 1, cti.LabelMalware: 1,
		"campaign": 1, "intrusion-set": 1, "attack-pattern": 1,
	}; !reflect.DeepEqual(got, want) {
		t.Errorf("dst labels = %v, want %v", got, want)
	}
	if got, want := kindCounts(dst), map[string]int{
		cti.EdgeBasedOn: 1, cti.EdgeIndicates: 1, cti.EdgeUses: 2, cti.EdgeAttributedTo: 1,
	}; !reflect.DeepEqual(got, want) {
		t.Errorf("dst edge kinds = %v, want %v", got, want)
	}

	obs := dst.GetNodesByLabel(cti.LabelObservable)
	if len(obs) != 1 {
		t.Fatalf("observables = %d, want 1", len(obs))
	}
	if want := map[string]any{"type": "domain", "value": "tutorial-c2.example.com", "name": "tutorial-c2.example.com", cti.SourceFeed: "case"}; !reflect.DeepEqual(obs[0].Props, want) {
		t.Errorf("observable props = %v, want %v", obs[0].Props, want)
	}

	inds := dst.GetNodesByLabel(cti.LabelIndicator)
	if len(inds) != 1 || inds[0].Props["name"] != "Tutorial C2" {
		t.Fatalf("dst indicators = %v, want only Tutorial C2", inds)
	}
	// Copied CTI nodes preserve Props exactly.
	srcInd := src.GetNodesByLabel(cti.LabelIndicator)
	var srcTutorial *struct{ props map[string]any }
	for _, n := range srcInd {
		if n.Props["name"] == "Tutorial C2" {
			srcTutorial = &struct{ props map[string]any }{n.Props}
		}
	}
	if srcTutorial == nil {
		t.Fatal("src missing Tutorial C2 indicator")
	}
	if !reflect.DeepEqual(inds[0].Props, srcTutorial.props) {
		t.Errorf("copied indicator props = %v, want src props %v", inds[0].Props, srcTutorial.props)
	}

	// based-on runs indicator -> observable.
	found := false
	for _, e := range dst.GetEdgesFrom(inds[0].ID) {
		if e.Kind == cti.EdgeBasedOn && e.To == obs[0].ID {
			found = true
		}
	}
	if !found {
		t.Error("no based-on edge from indicator to observable")
	}

	// src untouched.
	if got := len(src.GetAllNodes()); got != wantNodes {
		t.Errorf("src nodes = %d, want %d (mutated)", got, wantNodes)
	}
	if got := len(src.GetAllEdges()); got != wantEdges {
		t.Errorf("src edges = %d, want %d (mutated)", got, wantEdges)
	}
}

func TestBuildWorkspaceDedup(t *testing.T) {
	src := seedCorrelateGraph(t)
	dst := inmem.New()
	// Same domain twice (different context) plus the hash: 2 observable
	// nodes, both indicators, shared context copied once.
	caseObs := []CaseObservable{
		{Observable: Observable{Type: "domain", Value: "tutorial-c2.example.com"}, Context: "proxy"},
		{Observable: Observable{Type: "domain", Value: "Tutorial-C2.EXAMPLE.com."}, Context: "dns"},
		{Observable: Observable{Type: "file-hash-sha256", Value: "a1b2c3d4e5f60718293a4b5c6d7e8f90a1b2c3d4e5f60718293a4b5c6d7e8f90"}},
	}
	stats, err := BuildWorkspace(src, dst, caseObs, WorkspaceOptions{CaseSource: "case"})
	if err != nil {
		t.Fatalf("BuildWorkspace: %v", err)
	}
	if stats.Observables != 2 {
		t.Errorf("observables = %d, want 2 (domain deduped)", stats.Observables)
	}
	if stats.Indicators != 2 {
		t.Errorf("indicators = %d, want 2", stats.Indicators)
	}
	if got := len(dst.GetNodesByLabel(cti.LabelObservable)); got != 2 {
		t.Errorf("observable nodes = %d, want 2", got)
	}
	// One malware node shared by both indicators; based-on per pair.
	if got := len(dst.GetNodesByLabel(cti.LabelMalware)); got != 1 {
		t.Errorf("malware nodes = %d, want 1", got)
	}
	if got := kindCounts(dst)[cti.EdgeBasedOn]; got != 2 {
		t.Errorf("based-on edges = %d, want 2", got)
	}
}

func TestBuildWorkspaceHasBasedOn(t *testing.T) {
	src := seedCorrelateGraph(t)
	dst := inmem.New()
	caseObs := []CaseObservable{
		{Observable: Observable{Type: "domain", Value: "tutorial-c2.example.com"}},
		{Observable: Observable{Type: "file-hash-sha256", Value: "a1b2c3d4e5f60718293a4b5c6d7e8f90a1b2c3d4e5f60718293a4b5c6d7e8f90"}},
	}
	if _, err := BuildWorkspace(src, dst, caseObs, WorkspaceOptions{CaseSource: "case"}); err != nil {
		t.Fatalf("BuildWorkspace: %v", err)
	}

	// Wire the compiled-in CTI verbs (same as the CLI) and traverse.
	reg := extension.NewRegistry()
	if err := cti.New().Register(reg); err != nil {
		t.Fatalf("register: %v", err)
	}
	eng := graph.NewGraphEngine(dst)
	for name, v := range reg.Verbs() {
		eng.RegisterVerb(name, v)
	}

	res, err := eng.Find("observable").Exec(context.Background())
	if err != nil {
		t.Fatalf("find observable: %v", err)
	}
	if got := len(res.Items()); got != 2 {
		t.Fatalf("observable rows = %d, want 2", got)
	}

	res, err = eng.Find("indicator").Has("based-on", "tutorial-c2.example.com").Exec(context.Background())
	if err != nil {
		t.Fatalf("has based-on: %v", err)
	}
	rows := res.Items()
	if len(rows) != 1 {
		t.Fatalf("based-on rows = %d, want 1", len(rows))
	}
	// The row binds the traversal's nodes; the indicator must be among them.
	seenIndicator := false
	for _, n := range rows[0] {
		if name, _ := n.Props["name"].(string); name == "Tutorial C2" {
			seenIndicator = true
		}
	}
	if !seenIndicator {
		t.Errorf("based-on row %v missing Tutorial C2 indicator", rows[0])
	}
}

func TestBuildWorkspaceNoMatch(t *testing.T) {
	src := seedCorrelateGraph(t)
	dst := inmem.New()
	stats, err := BuildWorkspace(src, dst, []CaseObservable{
		{Observable: Observable{Type: "ipv4", Value: "203.0.113.9"}},
	}, WorkspaceOptions{CaseSource: "case"})
	if err != nil {
		t.Fatalf("BuildWorkspace: %v", err)
	}
	if stats != (WorkspaceStats{}) {
		t.Errorf("stats = %+v, want zero", stats)
	}
	if got := len(dst.GetAllNodes()); got != 0 {
		t.Errorf("dst nodes = %d, want 0", got)
	}
}

func TestBuildWorkspaceAllObservables(t *testing.T) {
	src := seedCorrelateGraph(t)
	// An indicator with no supported observable stays out of the layer.
	if _, err := src.AddNode("indicator", map[string]any{
		"name": "Weird", "pattern": "[file:name LIKE 'x%']",
		"stix_id": "indicator--99999999-9999-9999-9999-999999999999",
	}); err != nil {
		t.Fatalf("add weird indicator: %v", err)
	}
	wantNodes, wantEdges := len(src.GetAllNodes()), len(src.GetAllEdges())

	// Case matches the domain only; the hash layer comes from AllObservables.
	dst := inmem.New()
	stats, err := BuildWorkspace(src, dst, []CaseObservable{
		{Observable: Observable{Type: "domain", Value: "tutorial-c2.example.com"}},
	}, WorkspaceOptions{CaseSource: "case", AllObservables: true})
	if err != nil {
		t.Fatalf("BuildWorkspace: %v", err)
	}
	if got, want := stats, (WorkspaceStats{Observables: 2, Indicators: 2, Malware: 1, Campaigns: 1, Actors: 1, TTPs: 1, Edges: 7}); stats != want {
		t.Errorf("stats = %+v, want %+v", got, want)
	}

	// Both observable-bearing indicators present with both observables.
	if got := len(dst.GetNodesByLabel(cti.LabelIndicator)); got != 2 {
		t.Errorf("indicators = %d, want 2", got)
	}
	if got := len(dst.GetNodesByLabel(cti.LabelObservable)); got != 2 {
		t.Errorf("observables = %d, want 2", got)
	}
	for _, n := range dst.GetAllNodes() {
		if name, _ := n.Props["name"].(string); name == "Weird" {
			t.Errorf("observable-less indicator copied into dst: %v", n.ID)
		}
	}

	// Every based-on edge resolves on both ends (non-dangling).
	based := 0
	for _, e := range dst.GetAllEdges() {
		if e.Kind != cti.EdgeBasedOn {
			continue
		}
		based++
		if _, ok := dst.GetNode(e.From); !ok {
			t.Errorf("based-on edge %v dangles at from", e)
		}
		if _, ok := dst.GetNode(e.To); !ok {
			t.Errorf("based-on edge %v dangles at to", e)
		}
	}
	if based != 2 {
		t.Errorf("based-on edges = %d, want 2", based)
	}

	// src untouched.
	if got := len(src.GetAllNodes()); got != wantNodes {
		t.Errorf("src nodes = %d, want %d (mutated)", got, wantNodes)
	}
	if got := len(src.GetAllEdges()); got != wantEdges {
		t.Errorf("src edges = %d, want %d (mutated)", got, wantEdges)
	}
}
