// cmd/serve.go — DSL-over-HTTP server (issue #57, ADR 0010).
// One endpoint, stdlib net/http only, loopback by default. Both halves of
// a parsed program run through GraphEngine.Execute — the same path as
// the REPL — so the wire adds no semantics of its own.

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aprksy/knitknot/pkg/dsl"
	"github.com/aprksy/knitknot/pkg/graph"
	"github.com/aprksy/knitknot/pkg/ports/types"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Serve the DSL over HTTP (loopback by default)",
	Long: `Execute DSL programs over HTTP for out-of-process drivers (knitgen).

POST /v1/execute   {"dsl": "ADDNODE Person name='Ada'"}
                   → {"ok": true, "results": [...], "node_id": "..."}

No auth, no TLS: bind loopback only unless you know what you are doing.`,
	RunE: runServe,
}

var serveFlags struct {
	addr string
}

func init() {
	serveCmd.Flags().StringVar(&serveFlags.addr, "addr", "127.0.0.1:8080", "Address to listen on (loopback default; no auth exists)")
	RootCmd.AddCommand(serveCmd)
}

// executeRequest is the wire envelope. The DSL program is one statement
// (read or write); see dsl.ParseProgram.
type executeRequest struct {
	DSL string `json:"dsl"`
}

type nodeView struct {
	ID    string         `json:"id"`
	Label string         `json:"label"`
	Props map[string]any `json:"props"`
}

// executeResponse always carries results (possibly empty) so clients can
// read one field; write outcomes ride the id/verb fields (contract:
// graph.ExecuteResult).
type executeResponse struct {
	OK      bool                  `json:"ok"`
	Results []map[string]nodeView `json:"results"`
	NodeID  string                `json:"node_id,omitempty"`
	EdgeID  string                `json:"edge_id,omitempty"`
	Verb    string                `json:"verb,omitempty"`
	Error   string                `json:"error,omitempty"`
}

func runServe(cmd *cobra.Command, args []string) error {
	engine, err := LoadGraph(globalFlags.file)
	if err != nil {
		return err
	}
	if engine, err = resolveStoreBackend(engine); err != nil {
		return err
	}

	addr := serveFlags.addr
	if host, _, herr := net.SplitHostPort(addr); herr == nil && !isLoopback(host) {
		fmt.Fprintf(cmd.OutOrStdout(), "WARNING: --addr %q is not loopback and this server has no auth. Anyone who can reach it can rewrite your graph.\n", addr)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Serving DSL on http://%s (POST /v1/execute)\n", addr)
	return http.ListenAndServe(addr, newServeMux(engine))
}

func isLoopback(host string) bool {
	if host == "" || host == "localhost" {
		return true
	}
	if ip := net.ParseIP(strings.Trim(host, "[]")); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

// newServeMux wires the single endpoint; split out so tests drive it
// without binding a port.
func newServeMux(engine *graph.GraphEngine) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/execute", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeExecuteError(w, http.StatusMethodNotAllowed, "POST only")
			return
		}
		var req executeRequest
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			writeExecuteError(w, http.StatusBadRequest, "unreadable body")
			return
		}
		if err := json.Unmarshal(body, &req); err != nil || strings.TrimSpace(req.DSL) == "" {
			writeExecuteError(w, http.StatusBadRequest, "body must be JSON {\"dsl\": \"...\"}")
			return
		}
		prog, err := dsl.NewParser(req.DSL).ParseProgram()
		if err != nil {
			writeExecuteError(w, http.StatusBadRequest, fmt.Sprintf("parse error: %v", err))
			return
		}
		res, err := engine.Execute(context.Background(), prog)
		if err != nil {
			writeExecuteError(w, http.StatusBadRequest, err.Error())
			return
		}
		out, err := renderResult(res)
		if err != nil {
			writeExecuteError(w, http.StatusInternalServerError, fmt.Sprintf("render error: %v", err))
			return
		}
		writeExecuteJSON(w, http.StatusOK, out)
	})
	return mux
}

func renderResult(res *graph.ExecuteResult) (executeResponse, error) {
	out := executeResponse{OK: true, Results: []map[string]nodeView{}}
	if res.Query != nil {
		for _, row := range res.Query.Items() {
			view := make(map[string]nodeView, len(row))
			for v, n := range row {
				view[v] = toNodeView(n)
			}
			out.Results = append(out.Results, view)
		}
	}
	out.NodeID, out.EdgeID, out.Verb = res.NodeID, res.EdgeID, res.Verb
	return out, nil
}

func toNodeView(n *types.Node) nodeView {
	props := make(map[string]any, len(n.Props))
	for k, v := range n.Props {
		props[k] = v
	}
	return nodeView{ID: n.ID, Label: n.Label, Props: props}
}

func writeExecuteError(w http.ResponseWriter, status int, msg string) {
	writeExecuteJSON(w, status, executeResponse{OK: false, Results: []map[string]nodeView{}, Error: msg})
}

func writeExecuteJSON(w http.ResponseWriter, status int, out executeResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(out)
}
