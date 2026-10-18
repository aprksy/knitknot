package memory_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/aprksy/knitknot/extensions"
	"github.com/aprksy/knitknot/extensions/memory"
	"github.com/aprksy/knitknot/pkg/ports/types"
	"github.com/aprksy/knitknot/pkg/storage/inmem"
)

func importFixture(t *testing.T, s *inmem.Storage) {
	t.Helper()
	data, err := os.ReadFile("testdata/memory-bundle.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if err := memory.NewMemoryImporter().Import(context.Background(), extension.ImportContext{}, s, types.NewVerbRegistry(), strings.NewReader(string(data))); err != nil {
		t.Fatalf("import fixture: %v", err)
	}
}

func nodeByMemoryID(t *testing.T, s *inmem.Storage, id string) *types.Node {
	t.Helper()
	for _, n := range s.GetAllNodes() {
		if v, _ := n.Props[memory.PropMemoryID].(string); v == id {
			return n
		}
	}
	t.Fatalf("node %q not found", id)
	return nil
}

func TestImportNodesAndEdges(t *testing.T) {
	s := inmem.New()
	importFixture(t, s)
	if got := len(s.GetAllNodes()); got != 3 {
		t.Fatalf("nodes = %d, want 3", got)
	}
	edges := s.GetAllEdges()
	if len(edges) != 2 {
		t.Fatalf("edges = %d, want 2", len(edges))
	}
	turn := nodeByMemoryID(t, s, "turn-1")
	uprof := nodeByMemoryID(t, s, "u-alice")
	tag := nodeByMemoryID(t, s, "t-python")
	want := map[string]string{turn.ID + "\x00" + uprof.ID: "child-of", turn.ID + "\x00" + tag.ID: "tagged"}
	for _, e := range edges {
		k, ok := want[e.From+"\x00"+e.To]
		if !ok || k != e.Kind {
			t.Fatalf("unexpected edge %s -%s-> %s", e.From, e.Kind, e.To)
		}
	}
	if turn.Label != memory.LabelPRTurn {
		t.Fatalf("turn label = %q, want pr_turn", turn.Label)
	}
}

func TestUpsertMergeOnReimport(t *testing.T) {
	s := inmem.New()
	importFixture(t, s)
	re := `{
  "type": "memory-bundle", "version": 1,
  "nodes": [
    {"memory_id": "u-alice", "label": "user_profile", "props": {"user_id": "alice"}},
    {"memory_id": "turn-1", "label": "pr_turn", "props": {"user_id": "alice", "payload": {"prompt": "hello", "response": "hi again"}, "extra": "kept"}},
    {"memory_id": "t-python", "label": "tag", "props": {"name": "python"}}
  ],
  "edges": [
    {"from": "turn-1", "to": "u-alice", "kind": "child-of", "props": {}},
    {"from": "turn-1", "to": "t-python", "kind": "tagged", "props": {}}
  ]
}`
	if err := memory.NewMemoryImporter().Import(context.Background(), extension.ImportContext{}, s, types.NewVerbRegistry(), strings.NewReader(re)); err != nil {
		t.Fatalf("re-import: %v", err)
	}
	if got := len(s.GetAllNodes()); got != 3 {
		t.Fatalf("nodes = %d after re-import, want 3 (upsert, no dupes)", got)
	}
	if got := len(s.GetAllEdges()); got != 2 {
		t.Fatalf("edges = %d after re-import, want 2 (upsert, no dupes)", got)
	}
	turn := nodeByMemoryID(t, s, "turn-1")
	if v, _ := turn.Props["extra"].(string); v != "kept" {
		t.Fatalf("extra = %q, want kept (props merged)", v)
	}
	if len(turn.History) != 2 {
		t.Fatalf("turn history snapshots = %d, want 2 (history appended)", len(turn.History))
	}
}

func TestDanglingRefAbortsWithNoPartialWrites(t *testing.T) {
	s := inmem.New()
	bad := `{
  "type": "memory-bundle", "version": 1,
  "nodes": [{"memory_id": "turn-9", "label": "pr_turn", "props": {"user_id": "alice"}}],
  "edges": [{"from": "turn-9", "to": "ghost", "kind": "child-of", "props": {}}]
}`
	if err := memory.NewMemoryImporter().Import(context.Background(), extension.ImportContext{}, s, types.NewVerbRegistry(), strings.NewReader(bad)); err == nil {
		t.Fatal("expected error for dangling memory_id")
	}
	if got := len(s.GetAllNodes()); got != 0 {
		t.Fatalf("nodes = %d after failed import, want 0", got)
	}
	if got := len(s.GetAllEdges()); got != 0 {
		t.Fatalf("edges = %d after failed import, want 0", got)
	}
}

func TestUnknownLabelRejected(t *testing.T) {
	s := inmem.New()
	bad := `{
  "type": "memory-bundle", "version": 1,
  "nodes": [{"memory_id": "x-1", "label": "nope_label", "props": {}}],
  "edges": []
}`
	err := memory.NewMemoryImporter().Import(context.Background(), extension.ImportContext{}, s, types.NewVerbRegistry(), strings.NewReader(bad))
	if err == nil || !strings.Contains(err.Error(), "nope_label") {
		t.Fatalf("expected error naming the label, got %v", err)
	}
}

func TestUnknownKindRejected(t *testing.T) {
	s := inmem.New()
	bad := `{
  "type": "memory-bundle", "version": 1,
  "nodes": [{"memory_id": "x-1", "label": "tag", "props": {"name": "a"}}],
  "edges": [{"from": "x-1", "to": "x-1", "kind": "nope-kind", "props": {}}]
}`
	err := memory.NewMemoryImporter().Import(context.Background(), extension.ImportContext{}, s, types.NewVerbRegistry(), strings.NewReader(bad))
	if err == nil || !strings.Contains(err.Error(), "nope-kind") {
		t.Fatalf("expected error naming the kind, got %v", err)
	}
}

func TestSimilarToRequiresScoreAndModel(t *testing.T) {
	s := inmem.New()
	bad := `{
  "type": "memory-bundle", "version": 1,
  "nodes": [
    {"memory_id": "a", "label": "pr_turn", "props": {"user_id": "alice"}},
    {"memory_id": "b", "label": "pr_turn", "props": {"user_id": "alice"}}
  ],
  "edges": [{"from": "a", "to": "b", "kind": "similar-to", "props": {"score": 0.9}}]
}`
	err := memory.NewMemoryImporter().Import(context.Background(), extension.ImportContext{}, s, types.NewVerbRegistry(), strings.NewReader(bad))
	if err == nil || !strings.Contains(err.Error(), "model") {
		t.Fatalf("expected error requiring model prop, got %v", err)
	}
	if got := len(s.GetAllNodes()); got != 0 {
		t.Fatalf("nodes = %d after failed import, want 0", got)
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

func (f *fakeRegistry) Verbs() map[string]types.Verb { return f.verbs }

func (f *fakeRegistry) Freeze() {}

func TestExtensionRegisters(t *testing.T) {
	r := &fakeRegistry{verbs: map[string]types.Verb{}, importers: map[string]extension.Importer{}}
	if err := memory.New().Register(r); err != nil {
		t.Fatalf("register: %v", err)
	}
	if _, ok := r.Importer("memory-bundle"); !ok {
		t.Fatal("importer memory-bundle not registered")
	}
	for _, kind := range []string{"child-of", "similar-to", "supersedes", "tagged"} {
		if _, ok := r.verbs[kind]; !ok {
			t.Fatalf("verb %q not registered", kind)
		}
	}
	if v := r.verbs["tagged"]; v.TargetLabel != memory.LabelTag || v.MatchOn != memory.PropName {
		t.Fatalf("tagged verb = %+v, want target tag match on name", v)
	}
}
