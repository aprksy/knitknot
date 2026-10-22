package cmd

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/aprksy/knitknot/pkg/dsl"
	"github.com/aprksy/knitknot/pkg/graph"
)

func execDefine(engine *graph.GraphEngine, input string, out io.Writer) error {
	input = strings.TrimSpace(input)
	prog, err := dsl.NewParser(input).ParseProgram()
	if err != nil || prog.Write == nil || prog.Write.Op != dsl.WriteDefine {
		return fmt.Errorf("invalid syntax. Use: DEFINE <verb> TO <Label> VIA <property> (%v)", err)
	}

	if _, err := engine.Execute(context.Background(), prog); err != nil {
		return err
	}

	w := prog.Write
	fmt.Fprintf(out, "-- Verb '%s' defined: → %s.%s\n", w.VerbName, w.TargetLabel, w.MatchOn)
	return nil
}

func execListVerbs(engine *graph.GraphEngine, out io.Writer) error {
	verbs := engine.Verbs().All()

	if len(verbs) == 0 {
		fmt.Fprintln(out, "(no verbs defined)")
		return nil
	}

	// Find max width for alignment
	maxName := 0
	for name := range verbs {
		if len(name) > maxName {
			maxName = len(name)
		}
	}
	if maxName < 5 {
		maxName = 5
	}

	fmt.Fprintf(out, "%-*s → Target.Property\n", maxName, "VERB")
	fmt.Fprintln(out, strings.Repeat("-", maxName+20))
	for name, v := range verbs {
		prop := v.MatchOn
		if prop == "" {
			prop = "(any)"
		}
		target := v.TargetLabel
		if target == "" {
			target = "(any)"
		}
		fmt.Fprintf(out, "%-*s → %s.%s\n", maxName, name, target, prop)
	}
	return nil
}
