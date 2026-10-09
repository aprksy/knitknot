package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/aprksy/knitknot/extensions/cti"
	"github.com/aprksy/knitknot/pkg/dsl"
	"github.com/aprksy/knitknot/pkg/exporter/dot"
	"github.com/aprksy/knitknot/pkg/exporter/parquet"
	"github.com/aprksy/knitknot/pkg/graph"
	"github.com/aprksy/knitknot/pkg/ports/types"
	"github.com/aprksy/knitknot/pkg/storage/inmem"
	"github.com/spf13/cobra"
)

type ExportFormat string

const (
	FormatDOT     ExportFormat = "dot"
	FormatSVG     ExportFormat = "svg" // requires `dot` command
	FormatJSON    ExportFormat = "json"
	FormatSTIX    ExportFormat = "stix" // ext: cti-export
	FormatParquet ExportFormat = "parquet"
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export the graph in various formats",
	Long:  "Export the current graph state as DOT, SVG, JSON, STIX, or Parquet.",
	RunE:  runExport,
}

var exportFlags struct {
	format string
	output string
	query  string
}

func init() {
	exportCmd.Flags().StringVar(&globalFlags.subgraph, "subgraph", "", "Run query within a subgraph context")
	exportCmd.Flags().StringVarP(&exportFlags.format, "format", "F", "dot", "Output format (dot, svg, json, stix, parquet)") // ext: cti-export
	exportCmd.Flags().StringVarP(&exportFlags.output, "output", "o", "", "Output file (default stdout; prefix for parquet: writes <prefix>_nodes.parquet and <prefix>_edges.parquet)")
	exportCmd.Flags().StringVarP(&exportFlags.query, "query", "q", "", "Run a DSL query and export only the matching subgraph")
	RootCmd.AddCommand(exportCmd)
}

func runExport(cmd *cobra.Command, args []string) (err error) {
	format := ExportFormat(exportFlags.format)
	if format != FormatDOT && format != FormatSVG && format != FormatJSON && format != FormatSTIX && format != FormatParquet { // ext: cti-export
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

	// Parquet writes two files from the -o prefix, not a single file.
	if format == FormatParquet && exportFlags.output == "" {
		return fmt.Errorf("parquet export requires -o output prefix")
	}

	var writer io.Writer = os.Stdout
	if exportFlags.output != "" && format != FormatParquet {
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

	if exportFlags.query != "" {
		// Run the DSL query and collect the induced subgraph
		parser := dsl.NewParser(exportFlags.query)
		ast, err := parser.Parse()
		if err != nil {
			return fmt.Errorf("parse query: %w", err)
		}
		builder, err := ApplyAST(engine, ast)
		if err != nil {
			return fmt.Errorf("build query: %w", err)
		}
		result, err := builder.Exec(context.Background())
		if err != nil {
			return fmt.Errorf("exec query: %w", err)
		}

		// Collect all nodes from query results
		nodeSet := make(map[string]*types.Node)
		for _, row := range result.Items() {
			for _, n := range row {
				nodeSet[n.ID] = n
			}
		}
		for _, n := range nodeSet {
			nodes = append(nodes, n)
		}

		// Collect edges between result nodes (induced subgraph)
		nodeIDs := make(map[string]struct{}, len(nodeSet))
		for id := range nodeSet {
			nodeIDs[id] = struct{}{}
		}
		for _, e := range engine.Storage().GetAllEdges() {
			if _, ok := nodeIDs[e.From]; ok {
				if _, ok := nodeIDs[e.To]; ok {
					edges = append(edges, e)
				}
			}
		}

		// For STIX export, create a temp storage with just the subgraph
		if format == FormatSTIX {
			tmp := inmem.New()
			idMap := make(map[string]string, len(nodes))
			for _, n := range nodes {
				newID, err := tmp.AddNode(n.Label, n.Props)
				if err != nil {
					return fmt.Errorf("stage node: %w", err)
				}
				idMap[n.ID] = newID
			}
			for _, e := range edges {
				from := idMap[e.From]
				to := idMap[e.To]
				if err := tmp.AddEdge(from, to, e.Kind, e.Props); err != nil {
					return fmt.Errorf("stage edge: %w", err)
				}
			}
			engine = graph.NewGraphEngine(tmp)
		}
	} else if globalFlags.subgraph != "" {
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
	case FormatParquet:
		return exportToParquet(nodes, edges, exportFlags.output)
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

func exportToParquet(nodes []*types.Node, edges []*types.Edge, outputPrefix string) error {
	return parquet.ExportToParquet(nodes, edges, outputPrefix)
}

// exportToSTIX serializes the engine's storage as a STIX 2.1 bundle. // ext: cti-export
func exportToSTIX(engine *graph.GraphEngine, w io.Writer) error { // ext: cti-export
	return cti.NewSTIXExporter().Export(context.Background(), engine.Storage(), engine.Verbs(), w) // ext: cti-export
}
