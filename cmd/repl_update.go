package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/aprksy/knitknot/pkg/graph"
	"github.com/aprksy/knitknot/pkg/storage/inmem"
)

func execUpdate(engine *graph.GraphEngine, input string, out io.Writer) error {
	input = strings.TrimSpace(input)
	if !strings.HasPrefix(input, "NODE ") && !strings.HasPrefix(input, "EDGE ") {
		return fmt.Errorf("usage: UPDATE NODE <id> key=value... | UPDATE EDGE A --rel--> B key=value...")
	}

	if strings.HasPrefix(input, "NODE ") {
		return execUpdateNode(engine, input[5:], out)
	} else if strings.HasPrefix(input, "EDGE ") {
		return execUpdateEdge(engine, input[5:], out)
	}

	return fmt.Errorf("invalid UPDATE syntax")
}

func execUpdateNode(engine *graph.GraphEngine, input string, out io.Writer) error {
	fields := strings.Fields(input)
	if len(fields) < 1 {
		return fmt.Errorf("usage: UPDATE NODE <id> [key=value ...]")
	}

	id := fields[0]
	propsInput := ""
	if len(fields) > 1 {
		propsInput = strings.Join(fields[1:], " ")
	}

	node, ok := engine.GetNode(id)
	if !ok {
		return fmt.Errorf("node %s not found", id)
	}

	// Update properties
	newProps := make(map[string]any)
	for k, v := range node.Props {
		newProps[k] = v
	}

	updates := parseProps(propsInput)
	for k, v := range updates {
		newProps[k] = v
	}

	// In real system, you might have UpdateNode method
	// For now: rebuild storage layer doesn't support mutation → we'll simulate via re-add?
	// But better: expose mutable access safely

	// Since our storage is private, let's assume we can update
	storage, ok := engine.Storage().(*inmem.Storage)
	if !ok {
		return fmt.Errorf("storage not mutable")
	}

	storage.UpdateNode(id, newProps)
	fmt.Fprintf(out, "-- Updated node %s\n", id)
	return nil
}

func execUpdateEdge(engine *graph.GraphEngine, input string, out io.Writer) error {
	// Format: fromID --rel--> toID [props]
	arrowStart := strings.Index(input, "--")
	arrowEnd := strings.LastIndex(input, "-->")
	if arrowStart == -1 || arrowEnd == -1 || arrowEnd <= arrowStart {
		return fmt.Errorf("invalid edge format in UPDATE EDGE")
	}

	fromID := strings.TrimSpace(input[:arrowStart])
	middle := input[arrowStart+2 : arrowEnd]
	toID := strings.TrimSpace(input[arrowEnd+3:])

	rel := strings.TrimSpace(middle)
	rest := ""

	// If there are props after the arrow
	if space := strings.Index(rest, " "); space != -1 {
		// Extract props
		propsStr := strings.TrimSpace(rest[space+1:])
		if propsStr != "" {
			props := parseProps(propsStr)

			edgeID := fmt.Sprintf("%s->%s@%s", fromID, toID, rel)
			s, ok := engine.Storage().(*inmem.Storage)
			if !ok {
				return fmt.Errorf("storage not mutable")
			}

			edge, exists := engine.GetEdge(edgeID)
			if !exists {
				return fmt.Errorf("edge not found")
			}

			// Update props
			for k, v := range props {
				edge.Props[k] = v
			}

			s.UpdateEdge(edgeID, edge.Props)

			fmt.Fprintf(out, "-- Updated edge %s --%s--> %s\n", fromID, rel, toID)
			return nil
		}
	}

	return fmt.Errorf("no properties to update")
}
