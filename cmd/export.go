package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/aprksy/knitknot/extensions/cti"
	"github.com/aprksy/knitknot/pkg/exporter/dot"
	"github.com/aprksy/knitknot/pkg/graph"
	"github.com/aprksy/knitknot/pkg/ports/types"
	"github.com/spf13/cobra"
)

type ExportFormat string

const (
	FormatDOT  ExportFormat = "dot"
	FormatSVG  ExportFormat = "svg" // requires `dot` command
	FormatJSON ExportFormat = "json"
	FormatSTIX ExportFormat = "stix" // ext: cti-export
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export the graph in various formats",
	Long:  "Export the current graph state as DOT, SVG, JSON, or STIX.",
	RunE:  runExport,
}

var exportFlags struct {
	format string
	output string
}

func init() {
	exportCmd.Flags().StringVar(&globalFlags.subgraph, "subgraph", "", "Run query within a subgraph context")
	exportCmd.Flags().StringVarP(&exportFlags.format, "format", "F", "dot", "Output format (dot, svg, json, stix)") // ext: cti-export
	exportCmd.Flags().StringVarP(&exportFlags.output, "output", "o", "", "Output file (default stdout)")
	RootCmd.AddCommand(exportCmd)
}

func runExport(cmd *cobra.Command, args []string) (err error) {
	format := ExportFormat(exportFlags.format)
	if format != FormatDOT && format != FormatSVG && format != FormatJSON && format != FormatSTIX { // ext: cti-export
		return fmt.Errorf("unsupported format: %s", format)
	}

	// Load graph based on -f flag
	engine, err := LoadGraph(globalFlags.file)
	if err != nil {
		return err
	}
	if engine, err = resolveStoreBackend(engine); err != nil { // store: wire
		return err // store: wire
	}

	// if subgraph specified
	if globalFlags.subgraph != "" {
		engine = engine.WithSubgraph(globalFlags.subgraph)
	}

	var writer io.Writer = os.Stdout
	if exportFlags.output != "" {
		file, err := os.Create(exportFlags.output)
		if err != nil {
			return err
		}
		defer func() {
			if cerr := file.Close(); cerr != nil && err == nil {
				err = cerr
			}
		}()
		writer = file
	}

	var (
		nodes []*types.Node
		edges []*types.Edge
	)

	if globalFlags.subgraph != "" {
		nodes = engine.Storage().GetNodesIn(globalFlags.subgraph)
		edges = engine.Storage().GetEdgesIn(globalFlags.subgraph)
	} else {
		nodes = engine.Storage().GetAllNodes()
		edges = engine.Storage().GetAllEdges()
	}

	switch format {
	case FormatDOT:
		return exportToDOT(nodes, edges, writer)
	case FormatSVG:
		return exportToSVG(nodes, edges, writer)
	case FormatJSON:
		return exportToJSON(nodes, edges, engine.Verbs().All(), writer)
	case FormatSTIX: // ext: cti-export
		return exportToSTIX(engine, writer) // ext: cti-export
	}

	return nil
}

func exportToJSON(nodes []*types.Node, edges []*types.Edge, verbs map[string]types.Verb, w io.Writer) error {
	doc := struct {
		Nodes []*types.Node         `json:"nodes"`
		Edges []*types.Edge         `json:"edges"`
		Verbs map[string]types.Verb `json:"verbs"`
	}{nodes, edges, verbs}

	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("json marshal: %w", err)
	}
	_, err = w.Write(data)
	return err
}

func exportToSVG(nodes []*types.Node, edges []*types.Edge, w io.Writer) error {
	// Use external `dot` command
	cmd := exec.Command("dot", "-Tsvg")
	reader, writer := io.Pipe()
	cmd.Stdin = reader
	cmd.Stdout = w

	// Start dot
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("install graphviz: %w", err)
	}

	// Write DOT to pipe
	go func() {
		defer writer.Close()
		_ = dot.ExportToDOT(nodes, edges, writer)
	}()

	// Wait for completion
	return cmd.Wait()
}

func exportToDOT(nodes []*types.Node, edges []*types.Edge, w io.Writer) error {
	return dot.ExportToDOT(nodes, edges, w)
}

// exportToSTIX serializes the engine's storage as a STIX 2.1 bundle. // ext: cti-export
func exportToSTIX(engine *graph.GraphEngine, w io.Writer) error { // ext: cti-export
	return cti.NewSTIXExporter().Export(context.Background(), engine.Storage(), engine.Verbs(), w) // ext: cti-export
}
