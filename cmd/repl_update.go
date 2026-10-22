// REPL write handlers (ADR 0009 W3): routed through the pkg/dsl grammar
// and graph.Execute. Signatures and printed output stay the same; the
// inmem type-assert hack is gone — updates go through the engine, and
// storage keeps REPLACE semantics on the merged props the engine sends.
package cmd

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/aprksy/knitknot/pkg/dsl"
	"github.com/aprksy/knitknot/pkg/graph"
)

func execUpdate(engine *graph.GraphEngine, input string, out io.Writer) error {
	input = strings.TrimSpace(input)
	if !strings.HasPrefix(input, "NODE ") && !strings.HasPrefix(input, "EDGE ") {
		return fmt.Errorf("usage: UPDATE NODE <id> key=value... | UPDATE EDGE A --rel--> B key=value...")
	}

	if strings.HasPrefix(input, "NODE ") {
		return execUpdateNode(engine, input[5:], out)
	}
	return execUpdateEdge(engine, input[5:], out)
}

func execUpdateNode(engine *graph.GraphEngine, input string, out io.Writer) error {
	input = strings.TrimSpace(input)
	if input == "" {
		return fmt.Errorf("usage: UPDATE NODE <id> [key=value ...]")
	}

	prog, err := dsl.NewParser(dsl.WriteUpdate + " NODE " + input).ParseProgram()
	if err != nil {
		return fmt.Errorf("usage: UPDATE NODE <id> [key=value ...] (%v)", err)
	}
	if _, err := engine.Execute(context.Background(), prog); err != nil {
		return err
	}

	fmt.Fprintf(out, "-- Updated node %s\n", prog.Write.NodeID)
	return nil
}

func execUpdateEdge(engine *graph.GraphEngine, input string, out io.Writer) error {
	input = strings.TrimSpace(input)

	prog, err := dsl.NewParser(dsl.WriteUpdate + " EDGE " + input).ParseProgram()
	if err != nil {
		return fmt.Errorf("invalid edge format in UPDATE EDGE (%v)", err)
	}
	if len(prog.Write.Props) == 0 {
		return fmt.Errorf("no properties to update")
	}
	if _, err := engine.Execute(context.Background(), prog); err != nil {
		return err
	}

	e := prog.Write.Edge
	fmt.Fprintf(out, "-- Updated edge %s --%s--> %s\n", e.From, e.Rel, e.To)
	return nil
}
