package cmd

import (
	"fmt"
	"io"
	"strings"

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

		err := engine.DeleteNode(id)
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "-- Deleted node %s\n", id)
		return nil
	}

	if strings.HasPrefix(input, "EDGE ") {
		return execDeleteEdge(engine, input[5:], out)
	}

	return fmt.Errorf("invalid DELETE syntax")
}

func execDeleteEdge(engine *graph.GraphEngine, input string, out io.Writer) error {
	arrowStart := strings.Index(input, "--")
	arrowEnd := strings.LastIndex(input, "-->")
	if arrowStart == -1 || arrowEnd == -1 || arrowEnd <= arrowStart {
		return fmt.Errorf("invalid edge format")
	}

	fromID := strings.TrimSpace(input[:arrowStart])
	rel := strings.TrimSpace(input[arrowStart+2 : arrowEnd])
	toID := strings.TrimSpace(input[arrowEnd+3:])

	if fromID == "" || toID == "" || rel == "" {
		return fmt.Errorf("invalid edge spec")
	}

	err := engine.DeleteEdge(fromID, toID, rel)
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "-- Deleted edge %s --%s--> %s\n", fromID, rel, toID)
	return nil
}
