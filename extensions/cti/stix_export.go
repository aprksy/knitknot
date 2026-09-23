// ext: cti-export — STIX 2.1 bundle exporter (graph -> bundle).
package cti

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/TcM1911/stix2"
	"github.com/aprksy/knitknot/extensions"
	"github.com/aprksy/knitknot/pkg/ports/storage"
	"github.com/aprksy/knitknot/pkg/ports/types"
)

// STIXExporter serializes exportable graph state as a STIX 2.1 bundle. // ext: cti-export
type STIXExporter struct{} // ext: cti-export

// NewSTIXExporter returns a STIX 2.1 bundle exporter. // ext: cti-export
func NewSTIXExporter() *STIXExporter { return &STIXExporter{} } // ext: cti-export

var _ extension.Exporter = (*STIXExporter)(nil) // ext: cti-export

// Export writes the current graph state as a STIX 2.1 bundle document. // ext: cti-export
// Only nodes carrying a valid stix_id are exported; only edges carrying a // ext: cti-export
// valid relationship stix_id whose endpoints are both exported are exported. // ext: cti-export
// Anything else is skipped and counted, never fabricated. Verbs are not part // ext: cti-export
// of STIX output and are ignored. If anything was skipped, a plain-text // ext: cti-export
// "# knitknot skipped ..." summary line is appended after the bundle JSON. // ext: cti-export
func (e *STIXExporter) Export(ctx context.Context, s storage.StorageEngine, verbs *types.VerbRegistry, w io.Writer) error { // ext: cti-export
	_ = verbs                // ext: cti-export — verbs are not part of STIX output.
	nodes := s.GetAllNodes() // ext: cti-export
	edges := s.GetAllEdges() // ext: cti-export

	refOf := make(map[string]string, len(nodes)) // ext: cti-export — graph ID -> STIX ID.
	var objs []*stix2.CustomObject               // ext: cti-export
	skippedNodes, skippedEdges := 0, 0           // ext: cti-export
	for i, n := range nodes {                    // ext: cti-export
		if i%256 == 0 { // ext: cti-export
			if err := ctx.Err(); err != nil { // ext: cti-export
				return err // ext: cti-export
			} // ext: cti-export
		} // ext: cti-export
		obj, ok := exportNode(n) // ext: cti-export
		if !ok {                 // ext: cti-export
			skippedNodes++ // ext: cti-export
			continue       // ext: cti-export
		} // ext: cti-export
		refOf[n.ID] = string(obj.GetID()) // ext: cti-export
		objs = append(objs, obj)          // ext: cti-export
	} // ext: cti-export
	for i, e := range edges { // ext: cti-export
		if i%256 == 0 { // ext: cti-export
			if err := ctx.Err(); err != nil { // ext: cti-export
				return err // ext: cti-export
			} // ext: cti-export
		} // ext: cti-export
		fromRef, fromOK := refOf[e.From] // ext: cti-export
		toRef, toOK := refOf[e.To]       // ext: cti-export
		if !fromOK || !toOK {            // ext: cti-export
			skippedEdges++ // ext: cti-export
			continue       // ext: cti-export
		} // ext: cti-export
		obj, ok := exportEdge(e, fromRef, toRef) // ext: cti-export
		if !ok {                                 // ext: cti-export
			skippedEdges++ // ext: cti-export
			continue       // ext: cti-export
		} // ext: cti-export
		objs = append(objs, obj) // ext: cti-export
	} // ext: cti-export

	// ponytail: sort by STIX ID so output is deterministic despite unordered storage scans.
	sort.Slice(objs, func(i, j int) bool { return objs[i].GetID() < objs[j].GetID() }) // ext: cti-export

	col := stix2.New()         // ext: cti-export
	for _, obj := range objs { // ext: cti-export
		if err := col.Add(obj); err != nil { // ext: cti-export
			return fmt.Errorf("cti: export object %q: %w", obj.GetID(), err) // ext: cti-export
		} // ext: cti-export
	} // ext: cti-export
	bundle, err := col.ToBundle() // ext: cti-export — fresh bundle--<uuid> envelope.
	if err != nil {               // ext: cti-export
		return fmt.Errorf("cti: build bundle: %w", err) // ext: cti-export
	} // ext: cti-export
	// Bundle.Objects is omitempty: force the empty array so an empty graph is still a valid bundle.
	doc := struct { // ext: cti-export
		Type    string            `json:"type"`    // ext: cti-export
		ID      string            `json:"id"`      // ext: cti-export
		Objects []json.RawMessage `json:"objects"` // ext: cti-export
	}{Type: string(bundle.Type), ID: string(bundle.ID), Objects: bundle.Objects} // ext: cti-export
	if doc.Objects == nil { // ext: cti-export
		doc.Objects = []json.RawMessage{} // ext: cti-export
	} // ext: cti-export
	data, err := json.MarshalIndent(doc, "", "  ") // ext: cti-export
	if err != nil {                                // ext: cti-export
		return fmt.Errorf("cti: marshal bundle: %w", err) // ext: cti-export
	} // ext: cti-export
	if _, err := w.Write(data); err != nil { // ext: cti-export
		return err // ext: cti-export
	} // ext: cti-export
	if skippedNodes+skippedEdges > 0 { // ext: cti-export
		_, err := fmt.Fprintf(w, "\n# knitknot skipped %d nodes and %d edges (not stix-shaped or non-stix endpoints)\n", skippedNodes, skippedEdges) // ext: cti-export
		return err                                                                                                                                   // ext: cti-export
	} // ext: cti-export
	return nil // ext: cti-export
} // ext: cti-export

// exportNode rebuilds a STIX object map from node props verbatim. The stable // ext: cti-export
// stix_id becomes id and its type prefix becomes type; knitknot-internal // ext: cti-export
// props (source_feed, imported_at) are stripped. No typed constructors: the // ext: cti-export
// lib's CustomObject round-trips known and unknown types identically. // ext: cti-export
func exportNode(n *types.Node) (*stix2.CustomObject, bool) { // ext: cti-export
	sid, _ := n.Props[StixID].(string) // ext: cti-export
	if sid == "" {                     // ext: cti-export
		return nil, false // ext: cti-export — never fabricate STIX IDs for non-STIX nodes.
	} // ext: cti-export
	objType, _, err := ParseSTIXID(sid) // ext: cti-export
	if err != nil {                     // ext: cti-export
		return nil, false // ext: cti-export
	} // ext: cti-export
	obj := stix2.CustomObject(stripInternal(n.Props)) // ext: cti-export
	obj.Set("id", sid)                                // ext: cti-export
	obj.Set("type", objType)                          // ext: cti-export
	return &obj, true                                 // ext: cti-export
} // ext: cti-export

// exportEdge rebuilds a relationship object map from edge props. Endpoints // ext: cti-export
// come from live graph resolution (fromRef/toRef), not stale stored refs. A // ext: cti-export
// missing relationship_type defaults to the edge kind, which is how the // ext: cti-export
// importer stores it — that is the recorded value, not a fabrication. // ext: cti-export
func exportEdge(e *types.Edge, fromRef, toRef string) (*stix2.CustomObject, bool) { // ext: cti-export
	sid, _ := e.Props[StixID].(string) // ext: cti-export
	if sid == "" {                     // ext: cti-export
		return nil, false // ext: cti-export — never fabricate relationship IDs.
	} // ext: cti-export
	objType, _, err := ParseSTIXID(sid)          // ext: cti-export
	if err != nil || objType != "relationship" { // ext: cti-export
		return nil, false // ext: cti-export
	} // ext: cti-export
	obj := stix2.CustomObject(stripInternal(e.Props)) // ext: cti-export
	obj.Set("id", sid)                                // ext: cti-export
	obj.Set("type", "relationship")                   // ext: cti-export
	obj.Set("source_ref", fromRef)                    // ext: cti-export
	obj.Set("target_ref", toRef)                      // ext: cti-export
	if _, ok := obj["relationship_type"]; !ok {       // ext: cti-export
		obj.Set("relationship_type", e.Kind) // ext: cti-export
	} // ext: cti-export
	return &obj, true // ext: cti-export
} // ext: cti-export

// stripInternal copies props minus knitknot-side bookkeeping. // ext: cti-export
func stripInternal(props map[string]any) map[string]any { // ext: cti-export
	out := make(map[string]any, len(props)) // ext: cti-export
	for k, v := range props {               // ext: cti-export
		switch k { // ext: cti-export
		case StixID, SourceFeed, ImportedAt: // ext: cti-export
			continue // ext: cti-export
		} // ext: cti-export
		out[k] = v // ext: cti-export
	} // ext: cti-export
	return out // ext: cti-export
} // ext: cti-export
