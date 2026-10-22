package graph_test

// W2 executor tests (ADR 0009): create→query round-trips, ID contract,
// fail-before-mutate. Plain testing; ginkgo suite coexists.

import (
	"context"
	"testing"

	"github.com/aprksy/knitknot/pkg/dsl"
	"github.com/aprksy/knitknot/pkg/graph"
	"github.com/aprksy/knitknot/pkg/storage/inmem"
)

func newEngine(t *testing.T) *graph.GraphEngine {
	t.Helper()
	return graph.NewGraphEngine(inmem.New())
}

func mustProgram(t *testing.T, input string) *dsl.Program {
	t.Helper()
	prog, err := dsl.NewParser(input).ParseProgram()
	if err != nil {
		t.Fatalf("ParseProgram(%q): %v", input, err)
	}
	return prog
}

func mustExecute(t *testing.T, ge *graph.GraphEngine, input string) *graph.ExecuteResult {
	t.Helper()
	res, err := ge.Execute(context.Background(), mustProgram(t, input))
	if err != nil {
		t.Fatalf("Execute(%q): %v", input, err)
	}
	return res
}

func TestExecuteAddNodeReturnsID(t *testing.T) {
	ge := newEngine(t)
	res := mustExecute(t, ge, "ADDNODE Person name=Alice age=35")
	if res.NodeID == "" {
		t.Fatal("ADDNODE returned no NodeID (contract: writes return IDs)")
	}
	node, ok := ge.GetNode(res.NodeID)
	if !ok {
		t.Fatalf("node %q not found after ADDNODE", res.NodeID)
	}
	if node.Label != "Person" {
		t.Fatalf("label = %q, want Person", node.Label)
	}
	if node.Props["name"] != "Alice" || node.Props["age"] != 35 {
		t.Fatalf("props = %#v", node.Props)
	}
}

func TestExecuteQueryRoundTrip(t *testing.T) {
	ge := newEngine(t)
	mustExecute(t, ge, "ADDNODE Person name=Alice age=35")
	res := mustExecute(t, ge, "Find('Person').Where('n.name', '=', 'Alice')")
	if res.Query.Len() != 1 {
		t.Fatalf("query returned %d rows, want 1", res.Query.Len())
	}
	if res.NodeID != "" || res.EdgeID != "" || res.Verb != "" {
		t.Fatalf("read program must leave write fields zero: %#v", res)
	}
}

func TestExecuteConnectAndQuery(t *testing.T) {
	ge := newEngine(t)
	idA := mustExecute(t, ge, "ADDNODE Person name=Alice").NodeID
	idB := mustExecute(t, ge, "ADDNODE Skill name=Go").NodeID
	res := mustExecute(t, ge, "CONNECT "+idA+" --has_skill--> "+idB)
	wantID := idA + "->" + idB + "@has_skill"
	if res.EdgeID != wantID {
		t.Fatalf("EdgeID = %q, want %q", res.EdgeID, wantID)
	}
	if _, ok := ge.GetEdge(wantID); !ok {
		t.Fatalf("edge %q not found after CONNECT", wantID)
	}
	// verb semantics round-trip: Has finds the connected value via Define.
	mustExecute(t, ge, "DEFINE has_skill TO Skill VIA name")
	res2 := mustExecute(t, ge, "Find('Person').Has('has_skill', 'Go')")
	if res2.Query.Len() != 1 {
		t.Fatalf("Has query returned %d rows, want 1", res2.Query.Len())
	}
	if res2.Verb != "" {
		t.Fatalf("read program must leave Verb zero: %#v", res2)
	}
}

func TestExecuteConnectFailsBeforeMutating(t *testing.T) {
	ge := newEngine(t)
	idA := mustExecute(t, ge, "ADDNODE Person name=Alice").NodeID
	prog := mustProgram(t, "CONNECT "+idA+" --knows--> ghost")
	if _, err := ge.Execute(context.Background(), prog); err == nil {
		t.Fatal("CONNECT to missing node must fail")
	}
	if _, ok := ge.GetEdge(idA + "->ghost@knows"); ok {
		t.Fatal("edge was created despite the error (fail-before-mutate broken)")
	}
}

func TestExecuteUpdateNodeMerges(t *testing.T) {
	ge := newEngine(t)
	id := mustExecute(t, ge, "ADDNODE Person name=Alice age=35").NodeID
	mustExecute(t, ge, "UPDATE NODE "+id+" age=36 city=Boston")
	node, _ := ge.GetNode(id)
	if node.Props["name"] != "Alice" || node.Props["age"] != 36 || node.Props["city"] != "Boston" {
		t.Fatalf("merged props = %#v, want name=Alice age=36 city=Boston", node.Props)
	}
	if _, err := ge.Execute(context.Background(), mustProgram(t, "UPDATE NODE ghost age=1")); err == nil {
		t.Fatal("UPDATE on missing node must fail")
	}
}

func TestExecuteUpdateEdgeMerges(t *testing.T) {
	ge := newEngine(t)
	idA := mustExecute(t, ge, "ADDNODE Person name=Alice").NodeID
	idB := mustExecute(t, ge, "ADDNODE Person name=Bob").NodeID
	mustExecute(t, ge, "CONNECT "+idA+" --knows--> "+idB)
	mustExecute(t, ge, "UPDATE EDGE "+idA+" --knows--> "+idB+" since=2020")
	edge, _ := ge.GetEdge(idA + "->" + idB + "@knows")
	if edge.Props["since"] != 2020 {
		t.Fatalf("edge props = %#v", edge.Props)
	}
}

func TestExecuteDeleteNodeAndEdge(t *testing.T) {
	ge := newEngine(t)
	idA := mustExecute(t, ge, "ADDNODE Person name=Alice").NodeID
	idB := mustExecute(t, ge, "ADDNODE Person name=Bob").NodeID
	mustExecute(t, ge, "CONNECT "+idA+" --knows--> "+idB)

	res := mustExecute(t, ge, "DELETE EDGE "+idA+" --knows--> "+idB)
	if res.EdgeID != idA+"->"+idB+"@knows" {
		t.Fatalf("DELETE EDGE EdgeID = %q", res.EdgeID)
	}
	if _, err := ge.Execute(context.Background(), mustProgram(t, "DELETE EDGE "+idA+" --knows--> "+idB)); err == nil {
		t.Fatal("second DELETE EDGE must fail (already deleted)")
	}

	res2 := mustExecute(t, ge, "DELETE NODE "+idA)
	if res2.NodeID != idA {
		t.Fatalf("DELETE NODE NodeID = %q", res2.NodeID)
	}
	q := mustExecute(t, ge, "Find('Person')")
	if q.Query.Len() != 1 { // only Bob remains
		t.Fatalf("Find after delete returned %d rows, want 1", q.Query.Len())
	}
	if _, err := ge.Execute(context.Background(), mustProgram(t, "DELETE NODE "+idA)); err == nil {
		t.Fatal("second DELETE NODE must fail (already deleted)")
	}
}

func TestExecuteNilProgram(t *testing.T) {
	ge := newEngine(t)
	if _, err := ge.Execute(context.Background(), nil); err == nil {
		t.Fatal("nil program must error")
	}
}
