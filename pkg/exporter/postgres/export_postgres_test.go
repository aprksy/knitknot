package postgres

// Golden tests: exact script bytes — DDL, quoting, NULLs, inference.

import (
	"bytes"
	"testing"

	"github.com/aprksy/knitknot/pkg/ports/types"
)

func TestExportPostgresGolden(t *testing.T) {
	nodes := []*types.Node{
		{ID: "n2", Label: "Skill", Props: map[string]any{"name": "Go"}},
		{ID: "n1", Label: "Person", Props: map[string]any{"name": "O'Brien", "age": 36, "active": true}},
	}
	edges := []*types.Edge{
		{ID: "n1->n2@has_skill", From: "n1", To: "n2", Kind: "has_skill", Props: map[string]any{"since": 2020}},
	}
	var buf bytes.Buffer
	if err := ExportToPostgres(nodes, edges, &buf); err != nil {
		t.Fatalf("ExportToPostgres: %v", err)
	}
	want := `BEGIN;
CREATE TABLE "nodes" ("id" TEXT, "label" TEXT, "active" BOOLEAN, "age" INTEGER, "name" TEXT);
CREATE TABLE "edges" ("id" TEXT, "from_id" TEXT, "to_id" TEXT, "kind" TEXT, "since" INTEGER);
INSERT INTO "nodes" ("id", "label", "active", "age", "name") VALUES ('n1', 'Person', TRUE, 36, 'O''Brien');
INSERT INTO "nodes" ("id", "label", "active", "age", "name") VALUES ('n2', 'Skill', NULL, NULL, 'Go');
INSERT INTO "edges" ("id", "from_id", "to_id", "kind", "since") VALUES ('n1->n2@has_skill', 'n1', 'n2', 'has_skill', 2020);
COMMIT;
`
	if buf.String() != want {
		t.Fatalf("postgres script mismatch:\n got:\n%s\nwant:\n%s", buf.String(), want)
	}
}

func TestExportPostgresEmpty(t *testing.T) {
	var buf bytes.Buffer
	if err := ExportToPostgres(nil, nil, &buf); err != nil {
		t.Fatalf("ExportToPostgres: %v", err)
	}
	want := "BEGIN;\n" +
		"CREATE TABLE \"nodes\" (\"id\" TEXT, \"label\" TEXT);\n" +
		"CREATE TABLE \"edges\" (\"id\" TEXT, \"from_id\" TEXT, \"to_id\" TEXT, \"kind\" TEXT);\n" +
		"COMMIT;\n"
	if buf.String() != want {
		t.Fatalf("empty script mismatch:\n got: %q\nwant: %q", buf.String(), want)
	}
}
