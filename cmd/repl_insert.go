// REPL write handlers (ADR 0009 W3): routed through the pkg/dsl grammar
// and graph.Execute instead of ad-hoc string splitting. Signatures and
// printed output stay the same; syntax errors now carry the grammar's
// detail after the usage hint.
package cmd

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/aprksy/knitknot/pkg/dsl"
	"github.com/aprksy/knitknot/pkg/graph"
)

func execAddNode(engine *graph.GraphEngine, input string, out io.Writer) error {
	input = strings.TrimSpace(input)
	if input == "" {
		return fmt.Errorf("usage: ADDNODE Label [key=value ...]")
	}

	prog, err := dsl.NewParser(dsl.WriteAddNode + " " + input).ParseProgram()
	if err != nil {
		return fmt.Errorf("usage: ADDNODE Label [key=value ...] (%v)", err)
	}
	res, err := engine.Execute(context.Background(), prog)
	if err != nil {
		return err
	}

	label := prog.Write.Label
	fmt.Fprintf(out, "-- Created node: %s (%s)\n", res.NodeID, label)
	return nil
}

func execConnect(engine *graph.GraphEngine, input string, out io.Writer) error {
	input = strings.TrimSpace(input)
	// Formats: fromID --rel--> toID, or fromID --rel k=v--> toID (props
	// inside the arrow, matching help text).

	prog, err := dsl.NewParser(dsl.WriteConnect + " " + input).ParseProgram()
	if err != nil {
		return fmt.Errorf("invalid format. Use: FROM --rel--> TO (%v)", err)
	}

	if _, err := engine.Execute(context.Background(), prog); err != nil {
		return err
	}

	e := prog.Write.Edge
	fmt.Fprintf(out, "-- Connected %s --%s--> %s\n", e.From, e.Rel, e.To)
	return nil
}
