package cmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/aprksy/knitknot/pkg/graph"
	"github.com/aprksy/knitknot/pkg/ports/types"
	"github.com/aprksy/knitknot/pkg/storage/inmem"
	"github.com/spf13/cobra"
)

type ExportFormat string

const (
	FormatDOT  ExportFormat = "dot"
	FormatSVG  ExportFormat = "svg" // requires `dot` command
	FormatJSON ExportFormat = "json"
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export the graph in various formats",
	Long:  "Export the current graph state as DOT, SVG, or JSON.",
	RunE:  runExport,
}

var exportFlags struct {
	format string
	output string
}

func init() {
	exportCmd.Flags().StringVar(&globalFlags.subgraph, "subgraph", "", "Run query within a subgraph context")
	exportCmd.Flags().StringVarP(&exportFlags.format, "format", "f", "dot", "Output format (dot, svg, json)")
	exportCmd.Flags().StringVarP(&exportFlags.output, "output", "o", "", "Output file (default stdout)")
	RootCmd.AddCommand(exportCmd)
}

func runExport(cmd *cobra.Command, args []string) error {
	format := ExportFormat(exportFlags.format)
	if format != FormatDOT && format != FormatSVG && format != FormatJSON {
		return fmt.Errorf("unsupported format: %s", format)
	}

	// For now: create in-memory graph and populate sample data
	storage := inmem.New()
	engine := graph.NewGraphEngine(storage)

	seedSampleData(engine)

	var writer io.Writer = os.Stdout
	if exportFlags.output != "" {
		file, err := os.Create(exportFlags.output)
		if err != nil {
			return err
		}
		defer file.Close()
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
		// case FormatJSON:
		// 	return exportToJSON(engine, writer)
	}

	return nil
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
		_ = graph.ExportToDOT(nodes, edges, writer)
	}()

	// Wait for completion
	return cmd.Wait()
}

func exportToDOT(nodes []*types.Node, edges []*types.Edge, w io.Writer) error {
	return graph.ExportToDOT(nodes, edges, w)
}
