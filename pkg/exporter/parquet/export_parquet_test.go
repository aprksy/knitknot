package parquet

import (
	"os"
	"path/filepath"
	"testing"

	parquetgo "github.com/parquet-go/parquet-go"
	"github.com/aprksy/knitknot/pkg/ports/types"
)

func TestExportToParquetRoundTrip(t *testing.T) {
	nodes := []*types.Node{
		{ID: "n1", Label: "Tool", Props: map[string]any{
			"stix_id": "tool--1", "name": "mimikatz", "source_feed": "demo",
			"imported_at": "2024-01-01T00:00:00Z", "x_extra": "kept",
		}},
		{ID: "n2", Label: "Malware", Props: map[string]any{"stix_id": "malware--2"}},
	}
	edges := []*types.Edge{
		{ID: "e1", From: "n1", To: "n2", Kind: "uses", Props: map[string]any{"source_feed": "demo"}},
	}

	prefix := filepath.Join(t.TempDir(), "test")
	if err := ExportToParquet(nodes, edges, prefix); err != nil {
		t.Fatal(err)
	}

	nodeRows := readRows[NodeRow](t, prefix+"_nodes.parquet")
	if len(nodeRows) != 2 {
		t.Fatalf("got %d node rows", len(nodeRows))
	}
	byStix := map[string]NodeRow{}
	for _, r := range nodeRows {
		byStix[r.StixID] = r
	}
	mimi := byStix["tool--1"]
	if mimi.Label != "Tool" || mimi.Name != "mimikatz" || mimi.SourceFeed != "demo" || mimi.ImportedAt != "2024-01-01T00:00:00Z" {
		t.Fatalf("bad node row: %+v", mimi)
	}
	if mimi.PropsJSON != `{"x_extra":"kept"}` {
		t.Fatalf("bad props_json: %s", mimi.PropsJSON)
	}
	if byStix["malware--2"].PropsJSON != "{}" {
		t.Fatalf("expected empty props_json, got %s", byStix["malware--2"].PropsJSON)
	}

	edgeRows := readRows[EdgeRow](t, prefix+"_edges.parquet")
	if len(edgeRows) != 1 {
		t.Fatalf("got %d edge rows", len(edgeRows))
	}
	e := edgeRows[0]
	if e.FromStixID != "tool--1" || e.ToStixID != "malware--2" || e.Kind != "uses" || e.SourceFeed != "demo" {
		t.Fatalf("bad edge row: %+v", e)
	}
}

func readRows[T any](t *testing.T, path string) []T {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	rows, err := parquetgo.Read[T](f, fi.Size())
	if err != nil {
		t.Fatal(err)
	}
	return rows
}
