package cmd

// Phase 1 server tests (issue #57): drive newServeMux via httptest — no
// port binding. Every assertion checks content, never just the status.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aprksy/knitknot/pkg/graph"
	"github.com/aprksy/knitknot/pkg/storage/inmem"
)

func postDSL(t *testing.T, h http.Handler, dsl string) (int, executeResponse) {
	t.Helper()
	body, _ := json.Marshal(executeRequest{DSL: dsl})
	req := httptest.NewRequest(http.MethodPost, "/v1/execute", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var out executeResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("response is not JSON: %v\n%s", err, rec.Body.String())
	}
	return rec.Code, out
}

func TestServeRoundTrip(t *testing.T) {
	mux := newServeMux(graph.NewGraphEngine(inmem.New()))

	code, add := postDSL(t, mux, "ADDNODE Person name='Ada' age=36")
	if code != 200 || !add.OK || add.NodeID == "" {
		t.Fatalf("ADDNODE: code=%d out=%+v", code, add)
	}

	code, q := postDSL(t, mux, "Find('Person').Where('n.name', '=', 'Ada')")
	if code != 200 || !q.OK {
		t.Fatalf("query: code=%d out=%+v", code, q)
	}
	if len(q.Results) != 1 {
		t.Fatalf("want 1 row, got %+v", q.Results)
	}
	got := q.Results[0]["n"]
	if got.Label != "Person" || got.Props["name"] != "Ada" || got.ID != add.NodeID {
		t.Fatalf("row content wrong: %+v (want id %s)", got, add.NodeID)
	}
}

func TestServeMalformedDSL(t *testing.T) {
	mux := newServeMux(graph.NewGraphEngine(inmem.New()))
	code, out := postDSL(t, mux, "ADDNODE Person name=")
	if code != 400 || out.OK {
		t.Fatalf("want 400 ok:false, got code=%d out=%+v", code, out)
	}
	if out.Error == "" {
		t.Fatal("error envelope must explain")
	}
}

func TestServeDanglingWriteFailsCleanly(t *testing.T) {
	mux := newServeMux(graph.NewGraphEngine(inmem.New()))
	code, out := postDSL(t, mux, "CONNECT ghost1 --knows--> ghost2")
	if code != 400 || out.OK {
		t.Fatalf("want 400 ok:false, got code=%d out=%+v", code, out)
	}
	if !strings.Contains(out.Error, "not found") {
		t.Fatalf("error should name the missing node: %q", out.Error)
	}
}

func TestServeRejectsNonPost(t *testing.T) {
	mux := newServeMux(graph.NewGraphEngine(inmem.New()))
	req := httptest.NewRequest(http.MethodGet, "/v1/execute", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("want 405, got %d", rec.Code)
	}
}

func TestServeDefineReturnsVerb(t *testing.T) {
	mux := newServeMux(graph.NewGraphEngine(inmem.New()))
	code, out := postDSL(t, mux, "DEFINE has_skill TO Skill VIA name")
	if code != 200 || !out.OK || out.Verb != "has_skill" {
		t.Fatalf("DEFINE: code=%d out=%+v", code, out)
	}
}
