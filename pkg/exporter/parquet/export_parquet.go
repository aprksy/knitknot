package parquet

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/aprksy/knitknot/pkg/ports/types"
	parquetgo "github.com/parquet-go/parquet-go"
)

// NodeRow is the parquet schema for graph nodes.
type NodeRow struct {
	StixID     string `parquet:"stix_id"`
	Label      string `parquet:"label"`
	Name       string `parquet:"name"`
	SourceFeed string `parquet:"source_feed"`
	ImportedAt string `parquet:"imported_at"`
	PropsJSON  string `parquet:"props_json"`
}

// EdgeRow is the parquet schema for graph edges.
type EdgeRow struct {
	FromStixID string `parquet:"from_stix_id"`
	ToStixID   string `parquet:"to_stix_id"`
	Kind       string `parquet:"kind"`
	SourceFeed string `parquet:"source_feed"`
	PropsJSON  string `parquet:"props_json"`
}

// ExportToParquet writes nodes and edges as two parquet files:
// outputPrefix + "_nodes.parquet" and outputPrefix + "_edges.parquet".
func ExportToParquet(nodes []*types.Node, edges []*types.Edge, outputPrefix string) error {
	idToStix := make(map[string]string, len(nodes))
	nodeRows := make([]NodeRow, 0, len(nodes))
	for _, n := range nodes {
		row, err := nodeToRow(n)
		if err != nil {
			return err
		}
		nodeRows = append(nodeRows, row)
		idToStix[n.ID] = row.StixID
	}

	edgeRows := make([]EdgeRow, 0, len(edges))
	for _, e := range edges {
		row, err := edgeToRow(e, idToStix)
		if err != nil {
			return err
		}
		edgeRows = append(edgeRows, row)
	}

	if err := writeRows(outputPrefix+"_nodes.parquet", nodeRows); err != nil {
		return err
	}
	return writeRows(outputPrefix+"_edges.parquet", edgeRows)
}

func writeRows[T any](path string, rows []T) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := parquetgo.Write(f, rows); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return f.Close()
}

// nodeReserved are props promoted to columns; the rest go to PropsJSON.
var nodeReserved = map[string]bool{
	"stix_id": true, "name": true, "source_feed": true, "imported_at": true,
}

func nodeToRow(n *types.Node) (NodeRow, error) {
	rest, err := restJSON(n.Props, nodeReserved)
	if err != nil {
		return NodeRow{}, err
	}
	return NodeRow{
		StixID:     propString(n.Props["stix_id"]),
		Label:      n.Label,
		Name:       propString(n.Props["name"]),
		SourceFeed: propString(n.Props["source_feed"]),
		ImportedAt: propString(n.Props["imported_at"]),
		PropsJSON:  rest,
	}, nil
}

var edgeReserved = map[string]bool{"source_feed": true}

func edgeToRow(e *types.Edge, idToStix map[string]string) (EdgeRow, error) {
	rest, err := restJSON(e.Props, edgeReserved)
	if err != nil {
		return EdgeRow{}, err
	}
	return EdgeRow{
		FromStixID: resolveStix(idToStix, e.From),
		ToStixID:   resolveStix(idToStix, e.To),
		Kind:       e.Kind,
		SourceFeed: propString(e.Props["source_feed"]),
		PropsJSON:  rest,
	}, nil
}

// resolveStix maps a graph ID to its stix_id, falling back to the raw ID
// when the endpoint is not in the exported node set.
func resolveStix(idToStix map[string]string, id string) string {
	if stix, ok := idToStix[id]; ok && stix != "" {
		return stix
	}
	return id
}

func restJSON(props map[string]any, reserved map[string]bool) (string, error) {
	rest := make(map[string]any, len(props))
	for k, v := range props {
		if !reserved[k] {
			rest[k] = v
		}
	}
	if len(rest) == 0 {
		return "{}", nil
	}
	data, err := json.Marshal(rest)
	if err != nil {
		return "", fmt.Errorf("marshal props: %w", err)
	}
	return string(data), nil
}

func propString(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	// ponytail: time.Time formats as RFC3339 for the imported_at column.
	if t, ok := v.(time.Time); ok {
		return t.Format(time.RFC3339)
	}
	if s, ok := v.(fmt.Stringer); ok {
		return s.String()
	}
	return fmt.Sprintf("%v", v)
}
