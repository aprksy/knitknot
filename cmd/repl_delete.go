// REPL write handlers (ADR 0009 W3): routed through the pkg/dsl grammar
// and graph.Execute. Signatures and printed output stay the same.
package cmd

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/aprksy/knitknot/pkg/dsl"
	"github.com/aprksy/knitknot/pkg/graph"
)

func execDelete(engine *graph.GraphEngine, input string, out io.Writer) error {
	input = strings.TrimSpace(input)
	if !strings.HasPrefix(input, "NODE ") && !strings.HasPrefix(input, "EDGE ") {
		return fmt.Errorf("usage: DELETE NODE <id> | DELETE EDGE A --rel--> B")
	}

	if strings.HasPrefix(input, "NODE ") {
		id := strings.TrimSpace(input[5:])
		if id == "" {
			return fmt.Errorf("missing node ID")
		}

		prog, err := dsl.NewParser(dsl.WriteDelete + " NODE " + id).ParseProgram()
		if err != nil {
			return err
		}
		if _, err := engine.Execute(context.Background(), prog); err != nil {
			return err
		}
		fmt.Fprintf(out, "-- Deleted node %s\n", id)
		return nil
	}

	return execDeleteEdge(engine, strings.TrimSpace(input[5:]), out)
}

func execDeleteEdge(engine *graph.GraphEngine, input string, out io.Writer) error {
	input = strings.TrimSpace(input)

	prog, err := dsl.NewParser(dsl.WriteDelete + " EDGE " + input).ParseProgram()
	if err != nil {
		return fmt.Errorf("invalid edge format (%v)", err)
	}
	if _, err := engine.Execute(context.Background(), prog); err != nil {
		return err
	}

	e := prog.Write.Edge
	fmt.Fprintf(out, "-- Deleted edge %s --%s--> %s\n", e.From, e.Rel, e.To)
	return nil
}
