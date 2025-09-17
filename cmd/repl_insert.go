// cmd/repl_data.go
package cmd

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/aprksy/knitknot/pkg/graph"
)

// parseProps converts "name=Alice age=35" to map[string]any
func parseProps(input string) map[string]any {
	props := make(map[string]any)
	for _, kv := range strings.Fields(input) {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) == 2 {
			// Try int
			if v, err := strconv.Atoi(parts[1]); err == nil {
				props[parts[0]] = v
			} else {
				props[parts[0]] = parts[1]
			}
		}
	}
	return props
}

func execAddNode(engine *graph.GraphEngine, input string, out io.Writer) error {
	input = strings.TrimSpace(input)
	if input == "" {
		return fmt.Errorf("usage: ADDNODE Label [key=value ...]")
	}

	fields := strings.Fields(input)
	label := fields[0]
	var props map[string]any
	if len(fields) > 1 {
		props = parseProps(strings.Join(fields[1:], " "))
	}

	id, err := engine.AddNode(label, props)
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "-- Created node: %s (%s)\n", id, label)
	return nil
}

func execConnect(engine *graph.GraphEngine, input string, out io.Writer) error {
	input = strings.TrimSpace(input)
	// Simple format: fromID --rel--> toID
	// Or: fromID --rel prop=123--> toID

	arrowStart := strings.Index(input, "--")
	arrowEnd := strings.LastIndex(input, "-->")
	if arrowStart == -1 || arrowEnd == -1 || arrowEnd <= arrowStart {
		return fmt.Errorf("invalid format. Use: FROM --rel--> TO")
	}

	fromID := strings.TrimSpace(input[:arrowStart])
	middle := input[arrowStart+2 : arrowEnd]
	toID := strings.TrimSpace(input[arrowEnd+3:])

	// Extract rel and props
	middle = strings.TrimSpace(middle)
	if middle == "" {
		return fmt.Errorf("missing relationship type")
	}

	var rel string
	var propsStr string

	if space := strings.Index(middle, " "); space != -1 {
		rel = middle[:space]
		propsStr = middle[space+1:]
	} else {
		rel = middle
	}

	props := parseProps(propsStr)

	if fromID == "" || toID == "" || rel == "" {
		return fmt.Errorf("invalid connect syntax")
	}

	err := engine.AddEdge(fromID, toID, rel, props)
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "-- Connected %s --%s--> %s\n", fromID, rel, toID)
	return nil
}
