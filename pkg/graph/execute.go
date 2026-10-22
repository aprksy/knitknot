// pkg/graph/execute.go — write-statement executor (ADR 0009 W2).
// Single entry point beside the query path: reads compile through
// ApplyAST, writes go straight to the kernel mutation methods.
// Writes fail before mutating: dangling refs and missing targets error
// out with nothing changed (same semantics the importers use).

package graph

import (
	"context"
	"fmt"

	"github.com/aprksy/knitknot/pkg/dsl"
	"github.com/aprksy/knitknot/pkg/ports/query"
	"github.com/aprksy/knitknot/pkg/ports/types"
)

// ExecuteResult is the write contract (ADR 0009, Decision 3): it is
// frozen from day one, so keep additions additive only. Exactly one
// group of fields is set, matching the program's statement.
type ExecuteResult struct {
	// Query is the result set of a read program.
	Query query.ResultSet
	// NodeID: ADDNODE → the new node's id; UPDATE NODE / DELETE NODE →
	// the target id.
	NodeID string
	// EdgeID: CONNECT / UPDATE EDGE / DELETE EDGE → the edge id in
	// storage form "from->to@kind".
	EdgeID string
	// Verb: DEFINE → the registered verb name.
	Verb string
}

// Execute runs one parsed program against the engine: a write statement
// (executed now) or a read query (compiled and executed). ctx is kept
// for symmetry with the query path; writes are synchronous.
func (ge *GraphEngine) Execute(ctx context.Context, prog *dsl.Program) (*ExecuteResult, error) {
	if prog == nil {
		return nil, fmt.Errorf("nil program")
	}
	if prog.Write != nil {
		return ge.executeWrite(prog.Write)
	}
	if prog.Query == nil {
		return nil, fmt.Errorf("empty program")
	}
	builder, err := ApplyAST(ge, prog.Query)
	if err != nil {
		return nil, err
	}
	result, err := builder.Exec(ctx)
	if err != nil {
		return nil, err
	}
	return &ExecuteResult{Query: result}, nil
}

func (ge *GraphEngine) executeWrite(w *dsl.WriteStatement) (*ExecuteResult, error) {
	switch w.Op {
	case dsl.WriteAddNode:
		props, err := propMap(w.Props)
		if err != nil {
			return nil, err
		}
		id, err := ge.AddNode(w.Label, props)
		if err != nil {
			return nil, err
		}
		return &ExecuteResult{NodeID: id}, nil

	case dsl.WriteConnect:
		props, err := propMap(w.Props)
		if err != nil {
			return nil, err
		}
		e := w.Edge
		if e == nil {
			return nil, fmt.Errorf("CONNECT without an edge spec")
		}
		if err := ge.AddEdge(e.From, e.To, e.Rel, props); err != nil {
			return nil, err // storage already fails before mutating
		}
		return &ExecuteResult{EdgeID: edgeID(e.From, e.To, e.Rel)}, nil

	case dsl.WriteUpdate:
		if w.NodeID != "" {
			node, ok := ge.GetNode(w.NodeID)
			if !ok { // fail before mutating (REPL parity)
				return nil, fmt.Errorf("node %s not found", w.NodeID)
			}
			merged, err := mergedProps(node.Props, w.Props)
			if err != nil {
				return nil, err
			}
			if err := ge.UpdateNode(w.NodeID, merged); err != nil {
				return nil, err
			}
			return &ExecuteResult{NodeID: w.NodeID}, nil
		}
		e := w.Edge
		if e == nil {
			return nil, fmt.Errorf("UPDATE without a node or edge target")
		}
		id := edgeID(e.From, e.To, e.Rel)
		edge, ok := ge.GetEdge(id)
		if !ok {
			return nil, fmt.Errorf("edge not found: %s", id)
		}
		merged, err := mergedProps(edge.Props, w.Props)
		if err != nil {
			return nil, err
		}
		if err := ge.UpdateEdge(id, merged); err != nil {
			return nil, err
		}
		return &ExecuteResult{EdgeID: id}, nil

	case dsl.WriteDelete:
		if w.NodeID != "" {
			if err := ge.DeleteNode(w.NodeID); err != nil {
				return nil, err
			}
			return &ExecuteResult{NodeID: w.NodeID}, nil
		}
		e := w.Edge
		if e == nil {
			return nil, fmt.Errorf("DELETE without a node or edge target")
		}
		if err := ge.DeleteEdge(e.From, e.To, e.Rel); err != nil {
			return nil, err
		}
		return &ExecuteResult{EdgeID: edgeID(e.From, e.To, e.Rel)}, nil

	case dsl.WriteDefine:
		ge.RegisterVerb(w.VerbName, types.Verb{TargetLabel: w.TargetLabel, MatchOn: w.MatchOn})
		return &ExecuteResult{Verb: w.VerbName}, nil
	}
	return nil, fmt.Errorf("unknown write statement %q", w.Op)
}

// edgeID mirrors the storage-side edge id format ("from->to@kind").
func edgeID(from, to, kind string) string {
	return fmt.Sprintf("%s->%s@%s", from, to, kind)
}

// propMap flattens parsed key=value pairs; values are strings or ints.
func propMap(props []dsl.PropAssign) (map[string]any, error) {
	m := make(map[string]any, len(props))
	for _, p := range props {
		switch v := p.Value.(type) {
		case *dsl.StringLiteral:
			m[p.Key] = v.Value
		case *dsl.NumberLiteral:
			m[p.Key] = v.Value
		default:
			return nil, fmt.Errorf("unsupported property value for %q", p.Key)
		}
	}
	return m, nil
}

// mergedProps copies old and overlays the parsed updates (REPL parity:
// UPDATE merges into existing props; storage keeps REPLACE semantics on
// the snapshot it records).
func mergedProps(old map[string]any, updates []dsl.PropAssign) (map[string]any, error) {
	merged := make(map[string]any, len(old)+len(updates))
	for k, v := range old {
		merged[k] = v
	}
	props, err := propMap(updates)
	if err != nil {
		return nil, err
	}
	for k, v := range props {
		merged[k] = v
	}
	return merged, nil
}
