package cmd

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/aprksy/knitknot/pkg/graph"
	"github.com/aprksy/knitknot/pkg/ports/types"
)

// definePattern matches: DEFINE has_skill TO Skill VIA name
var defineRegex = regexp.MustCompile(`(?i)^define\s+(\w+)\s+to\s+(\w+)\s+via\s+(\w+)$`)

func execDefine(engine *graph.GraphEngine, input string, out io.Writer) error {
	input = strings.TrimSpace(input)
	matches := defineRegex.FindStringSubmatch(input)
	fmt.Printf("%s; matches: %v\n", input, matches)
	if len(matches) != 4 {
		return fmt.Errorf("invalid syntax. Use: DEFINE <verb> TO <Label> VIA <property>")
	}

	verbName := matches[1]
	targetLabel := matches[2]
	matchOn := matches[3]

	def := types.Verb{
		TargetLabel: targetLabel,
		MatchOn:     matchOn,
	}

	engine.RegisterVerb(verbName, def)
	fmt.Fprintf(out, "-- Verb '%s' defined: → %s.%s\n", verbName, targetLabel, matchOn)
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
		fmt.Fprintf(out, "%-*s → %s.%s\n", maxName, name, v.TargetLabel, prop)
	}
	return nil
}
