package csv

// Golden tests: exact bytes for headers, rows, quoting, and the
// missing-prop rule. The format is frozen by these tests.

import (
	"bytes"
	"testing"

	"github.com/aprksy/knitknot/pkg/ports/types"
)

func TestExportNodesGolden(t *testing.T) {
	nodes := []*types.Node{
		{ID: "n2", Label: "Skill", Props: map[string]any{"name": "Go"}},
		{ID: "n1", Label: "Person", Props: map[string]any{"name": "Ada, \"the\" founder", "age": 36}},
	}
	var buf bytes.Buffer
	if err := ExportNodes(nodes, &buf); err != nil {
		t.Fatalf("ExportNodes: %v", err)
	}
	want := "id,label,age,name\n" +
		"n1,Person,36,\"Ada, \"\"the\"\" founder\"\n" +
		"n2,Skill,,Go\n"
	if buf.String() != want {
		t.Fatalf("nodes csv mismatch:\n got: %q\nwant: %q", buf.String(), want)
	}
}

func TestExportEdgesGolden(t *testing.T) {
	edges := []*types.Edge{
		{ID: "n1->n2@has_skill", From: "n1", To: "n2", Kind: "has_skill", Props: map[string]any{"since": 2020}},
		{ID: "n1->n3@knows", From: "n1", To: "n3", Kind: "knows", Props: nil},
	}
	var buf bytes.Buffer
	if err := ExportEdges(edges, &buf); err != nil {
		t.Fatalf("ExportEdges: %v", err)
	}
	want := "id,from,to,kind,since\n" +
		"n1->n2@has_skill,n1,n2,has_skill,2020\n" +
		"n1->n3@knows,n1,n3,knows,\n"
	if buf.String() != want {
		t.Fatalf("edges csv mismatch:\n got: %q\nwant: %q", buf.String(), want)
	}
}

func TestExportEmpty(t *testing.T) {
	var buf bytes.Buffer
	if err := ExportNodes(nil, &buf); err != nil {
		t.Fatalf("ExportNodes: %v", err)
	}
	if buf.String() != "id,label\n" {
		t.Fatalf("empty nodes header = %q", buf.String())
	}
	buf.Reset()
	if err := ExportEdges(nil, &buf); err != nil {
		t.Fatalf("ExportEdges: %v", err)
	}
	if buf.String() != "id,from,to,kind\n" {
		t.Fatalf("empty edges header = %q", buf.String())
	}
}
