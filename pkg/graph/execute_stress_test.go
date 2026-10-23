package graph_test

// Stretch test for the write DSL (ADR 0009): push the parse+execute path
// at volume in-process. Skipped under -short; run with:
//
//	go test ./pkg/graph/ -run TestExecuteStress -v
//
// Reports nodes/sec and edges/sec via t.Log. Asserts final counts, a
// sampled round-trip, and that a batch with dangling refs mutates nothing
// (fail-before-mutate must hold under load, not just in unit tests).

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aprksy/knitknot/pkg/dsl"
	"github.com/aprksy/knitknot/pkg/graph"
	"github.com/aprksy/knitknot/pkg/storage/inmem"
)

const (
	stressNodes = 10000
	stressEdges = 20000
)

func execLine(t *testing.T, ge *graph.GraphEngine, line string) (*graph.ExecuteResult, error) {
	prog, err := dsl.NewParser(line).ParseProgram()
	if err != nil {
		return nil, fmt.Errorf("parse %q: %w", line, err)
	}
	return ge.Execute(context.Background(), prog)
}

func TestExecuteStressWriteThroughput(t *testing.T) {
	if testing.Short() {
		t.Skip("stretch test; run without -short")
	}
	ge := graph.NewGraphEngine(inmem.New())

	// Skills and companies first, so edges can reference them.
	for s := 0; s < 50; s++ {
		if _, err := execLine(t, ge, fmt.Sprintf("ADDNODE Skill name=skill%03d level=%d", s, s%5+1)); err != nil {
			t.Fatalf("ADDNODE Skill: %v", err)
		}
	}
	for c := 0; c < 100; c++ {
		if _, err := execLine(t, ge, fmt.Sprintf("ADDNODE Company name=corp%03d employees=%d", c, 10+c)); err != nil {
			t.Fatalf("ADDNODE Company: %v", err)
		}
	}

	// N person nodes through parse + execute (the full grammar path).
	t0 := time.Now()
	personIDs := make([]string, 0, stressNodes)
	for p := 0; p < stressNodes; p++ {
		res, err := execLine(t, ge, fmt.Sprintf("ADDNODE Person name=pers%05d age=%d city=c%02d", p, 20+p%40, p%50))
		if err != nil {
			t.Fatalf("ADDNODE Person %d: %v", p, err)
		}
		personIDs = append(personIDs, res.NodeID)
	}
	nodeDur := time.Since(t0)

	// Edges: know-for-each-person (10K unique pairs), then
	// has_skill/works_at (10K more, unique pairs per phase — storage
	// overwrites duplicate from->to@kind ids silently, so no repeats).
	t0 = time.Now()
	for e := 0; e < stressEdges; e++ {
		src := personIDs[e%stressNodes]
		var line string
		if e < stressNodes { // phase 1: social layer, one edge per person
			line = fmt.Sprintf("CONNECT %s --knows weight=%d--> %s", src, e%9, personIDs[(e*7+3)%stressNodes])
		} else if e%2 == 0 {
			line = fmt.Sprintf("CONNECT %s --has_skill--> n%d", src, e%50+1)
		} else {
			line = fmt.Sprintf("CONNECT %s --works_at--> n%d", src, e%100+51)
		}
		if _, err := execLine(t, ge, line); err != nil {
			t.Fatalf("CONNECT %d: %v", e, err)
		}
	}
	edgeDur := time.Since(t0)
	t.Logf("nodes: %d in %v (%.0f/s); edges: %d in %v (%.0f/s)",
		stressNodes+150, nodeDur, float64(stressNodes+150)/nodeDur.Seconds(),
		stressEdges, edgeDur, float64(stressEdges)/edgeDur.Seconds())

	// Verify knowledge is correct at scale, without huge memory: label counts.
	persons := ge.Storage().GetNodesByLabel("Person")
	if len(persons) != stressNodes {
		t.Fatalf("Person count = %d, want %d", len(persons), stressNodes)
	}
	skills := ge.Storage().GetNodesByLabel("Skill")
	if len(skills) != 50 {
		t.Fatalf("Skill count = %d, want 50", len(skills))
	}
	edges := ge.Storage().GetAllEdges()
	if len(edges) != stressEdges {
		t.Fatalf("edge count = %d, want %d", len(edges), stressEdges)
	}

	// Sampled semantic round-trip (Has verb semantics through the grammar).
	if _, err := execLine(t, ge, "DEFINE has_skill TO Skill VIA name"); err != nil {
		t.Fatalf("DEFINE: %v", err)
	}
	q, err := execLine(t, ge, "Find('Person').Has('has_skill', 'skill000')")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if q.Query.Len() == 0 {
		t.Fatal("Has round-trip found nothing after 20K edges")
	}

	// UPDATE a random slice; props must merge correctly.
	for u := 0; u < 500; u++ {
		id := personIDs[(u*37)%stressNodes]
		if _, err := execLine(t, ge, fmt.Sprintf("UPDATE NODE %s verified=true", id)); err != nil {
			t.Fatalf("UPDATE %d: %v", u, err)
		}
	}
	n, ok := ge.GetNode(personIDs[37])
	if !ok || n.Props["verified"] != "true" || n.Props["name"] != "pers00037" {
		t.Fatalf("merged props = %#v", n.Props)
	}

	// Fail-before-mutate under volume: one bad line in a batch may not
	// create anything (the node the dangling edge references is missing).
	before := len(ge.Storage().GetAllEdges())
	bad := fmt.Sprintf("CONNECT %s --knows--> ghost-%d", personIDs[0], stressNodes)
	if _, err := execLine(t, ge, bad); err == nil {
		t.Fatal("dangling CONNECT must fail")
	}
	if len(ge.Storage().GetAllEdges()) != before {
		t.Fatal("failed CONNECT mutated storage")
	}
}
