// ext: cti-import — STIX 2.1 bundle importer (two-pass: stage, then upsert).
package cti

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/TcM1911/stix2"
	"github.com/aprksy/knitknot/extensions"
	"github.com/aprksy/knitknot/pkg/ports/storage"
	"github.com/aprksy/knitknot/pkg/ports/types"
)

// STIXImporter imports STIX 2.1 bundles into the graph. // ext: cti-import
type STIXImporter struct{} // ext: cti-import

// NewSTIXImporter returns a STIX 2.1 bundle importer. // ext: cti-import
func NewSTIXImporter() *STIXImporter { return &STIXImporter{} } // ext: cti-import

var _ extension.Importer = (*STIXImporter)(nil) // ext: cti-import

type stagedNode struct { // ext: cti-import
	stixID string         // ext: cti-import
	label  string         // ext: cti-import
	props  map[string]any // ext: cti-import
} // ext: cti-import

type stagedRel struct { // ext: cti-import
	stixID  string         // ext: cti-import
	kind    string         // ext: cti-import
	fromRef string         // ext: cti-import
	toRef   string         // ext: cti-import
	props   map[string]any // ext: cti-import
} // ext: cti-import

// Import reads a STIX 2.1 bundle and upserts its objects as nodes and edges. // ext: cti-import
// Pass 1 stages and validates everything without touching storage; pass 2 // ext: cti-import
// upserts nodes first, then resolves and upserts relationships. // ext: cti-import
func (im *STIXImporter) Import(ctx context.Context, s storage.StorageEngine, verbs *types.VerbRegistry, r io.Reader) error { // ext: cti-import
	_ = verbs                  // ext: cti-import
	data, err := io.ReadAll(r) // ext: cti-import
	if err != nil {            // ext: cti-import
		return fmt.Errorf("cti: read bundle: %w", err) // ext: cti-import
	} // ext: cti-import
	collection, err := stix2.FromJSON(data) // ext: cti-import
	if err != nil {                         // ext: cti-import
		return fmt.Errorf("cti: parse STIX bundle: %w", err) // ext: cti-import
	} // ext: cti-import

	// Pass 1: stage and validate, no storage mutation. // ext: cti-import
	var nodes []stagedNode                        // ext: cti-import
	var rels []stagedRel                          // ext: cti-import
	staged := make(map[string]struct{})           // ext: cti-import
	for i, obj := range collection.AllObjects() { // ext: cti-import
		if i%64 == 0 { // ext: cti-import
			if err := ctx.Err(); err != nil { // ext: cti-import
				return err // ext: cti-import
			} // ext: cti-import
		} // ext: cti-import
		stixID := string(obj.GetID()) // ext: cti-import
		if stixID == "" {             // ext: cti-import
			return fmt.Errorf("cti: object missing STIX id") // ext: cti-import
		} // ext: cti-import
		if _, _, err := ParseSTIXID(stixID); err != nil { // ext: cti-import
			return fmt.Errorf("cti: invalid STIX id %q: %w", stixID, err) // ext: cti-import
		} // ext: cti-import
		if rel, ok := obj.(*stix2.Relationship); ok { // ext: cti-import
			if rel.Source == "" || rel.Target == "" { // ext: cti-import
				return fmt.Errorf("cti: relationship %q missing source_ref or target_ref", stixID) // ext: cti-import
			} // ext: cti-import
			if rel.RelationshipType == "" { // ext: cti-import
				return fmt.Errorf("cti: relationship %q missing relationship_type", stixID) // ext: cti-import
			} // ext: cti-import
			props, err := graphProps(obj) // ext: cti-import
			if err != nil {               // ext: cti-import
				return fmt.Errorf("cti: relationship %q: %w", stixID, err) // ext: cti-import
			} // ext: cti-import
			rels = append(rels, stagedRel{stixID: stixID, kind: string(rel.RelationshipType), fromRef: string(rel.Source), toRef: string(rel.Target), props: props}) // ext: cti-import
			continue                                                                                                                                                 // ext: cti-import
		} // ext: cti-import
		if ind, ok := obj.(*stix2.Indicator); ok { // ext: cti-import
			if ind.Pattern == "" || ind.PatternType == "" || ind.ValidFrom == nil { // ext: cti-import
				return fmt.Errorf("cti: indicator %q missing required pattern, pattern_type, or valid_from", stixID) // ext: cti-import
			} // ext: cti-import
		} // ext: cti-import
		props, err := graphProps(obj) // ext: cti-import
		if err != nil {               // ext: cti-import
			return fmt.Errorf("cti: object %q: %w", stixID, err) // ext: cti-import
		} // ext: cti-import
		// ext: cti-import — label is the STIX type verbatim; supported SDOs match vocabulary consts, anything else (SCOs, x-custom) is preserved as-is.
		nodes = append(nodes, stagedNode{stixID: stixID, label: string(obj.GetType()), props: props}) // ext: cti-import
		staged[stixID] = struct{}{}                                                                   // ext: cti-import
	} // ext: cti-import

	// Still pass 1: every relationship endpoint must resolve against the staged // ext: cti-import
	// set or pre-existing storage before any mutation happens. // ext: cti-import
	for _, rel := range rels { // ext: cti-import
		for _, ref := range []string{rel.fromRef, rel.toRef} { // ext: cti-import
			if _, ok := staged[ref]; ok { // ext: cti-import
				continue // ext: cti-import
			} // ext: cti-import
			if findGraphID(s, ref) == "" { // ext: cti-import
				return fmt.Errorf("cti: relationship %q references unknown object %q", rel.stixID, ref) // ext: cti-import
			} // ext: cti-import
		} // ext: cti-import
	} // ext: cti-import

	// Pass 2: upsert nodes, then resolve and upsert relationships. // ext: cti-import
	// ponytail: single-writer lookup-before-add; needs a unique index for concurrent/large feeds.
	ids := make(map[string]string, len(nodes)) // ext: cti-import
	for i, n := range nodes {                  // ext: cti-import
		if i%64 == 0 { // ext: cti-import
			if err := ctx.Err(); err != nil { // ext: cti-import
				return err // ext: cti-import
			} // ext: cti-import
		} // ext: cti-import
		if existing := findNodeByLabel(s, n.label, n.stixID); existing != nil { // ext: cti-import
			if err := s.UpdateNode(existing.ID, mergeProps(existing.Props, n.props)); err != nil { // ext: cti-import
				return fmt.Errorf("cti: update node %q: %w", n.stixID, err) // ext: cti-import
			} // ext: cti-import
			ids[n.stixID] = existing.ID // ext: cti-import
			continue                    // ext: cti-import
		} // ext: cti-import
		gid, err := s.AddNode(n.label, n.props) // ext: cti-import
		if err != nil {                         // ext: cti-import
			return fmt.Errorf("cti: add node %q: %w", n.stixID, err) // ext: cti-import
		} // ext: cti-import
		ids[n.stixID] = gid // ext: cti-import
	} // ext: cti-import
	for i, rel := range rels { // ext: cti-import
		if i%64 == 0 { // ext: cti-import
			if err := ctx.Err(); err != nil { // ext: cti-import
				return err // ext: cti-import
			} // ext: cti-import
		} // ext: cti-import
		from := ids[rel.fromRef] // ext: cti-import
		if from == "" {          // ext: cti-import
			from = findGraphID(s, rel.fromRef) // ext: cti-import
		} // ext: cti-import
		to := ids[rel.toRef] // ext: cti-import
		if to == "" {        // ext: cti-import
			to = findGraphID(s, rel.toRef) // ext: cti-import
		} // ext: cti-import
		if from == "" || to == "" { // ext: cti-import
			return fmt.Errorf("cti: relationship %q has unresolvable endpoint", rel.stixID) // ext: cti-import
		} // ext: cti-import
		var found *types.Edge                    // ext: cti-import
		for _, e := range s.GetEdgesFrom(from) { // ext: cti-import
			if e.To == to && e.Kind == rel.kind { // ext: cti-import
				found = e // ext: cti-import
				break     // ext: cti-import
			} // ext: cti-import
		} // ext: cti-import
		if found != nil { // ext: cti-import
			if err := s.UpdateEdge(found.ID, mergeProps(found.Props, rel.props)); err != nil { // ext: cti-import
				return fmt.Errorf("cti: update edge %q: %w", rel.stixID, err) // ext: cti-import
			} // ext: cti-import
			continue // ext: cti-import
		} // ext: cti-import
		if err := s.AddEdge(from, to, rel.kind, rel.props); err != nil { // ext: cti-import
			return fmt.Errorf("cti: add edge %q: %w", rel.stixID, err) // ext: cti-import
		} // ext: cti-import
	} // ext: cti-import
	return nil // ext: cti-import
} // ext: cti-import

// graphProps flattens a STIX object to generic node/edge props via its JSON // ext: cti-import
// form, so known fields and raw custom properties survive without a // ext: cti-import
// per-type field list. // ext: cti-import
func graphProps(obj stix2.STIXObject) (map[string]any, error) { // ext: cti-import
	raw, err := json.Marshal(obj) // ext: cti-import
	if err != nil {               // ext: cti-import
		return nil, err // ext: cti-import
	} // ext: cti-import
	props := make(map[string]any)                       // ext: cti-import
	if err := json.Unmarshal(raw, &props); err != nil { // ext: cti-import
		return nil, err // ext: cti-import
	} // ext: cti-import
	props[StixID] = string(obj.GetID())                       // ext: cti-import
	delete(props, "id")                                       // ext: cti-import
	props[SourceFeed] = ""                                    // ext: cti-import
	props[ImportedAt] = time.Now().UTC().Format(time.RFC3339) // ext: cti-import
	return props, nil                                         // ext: cti-import
} // ext: cti-import

// findNodeByLabel returns the node with label carrying stixID, or nil. // ext: cti-import
func findNodeByLabel(s storage.StorageEngine, label, stixID string) *types.Node { // ext: cti-import
	for _, n := range s.GetNodesByLabel(label) { // ext: cti-import
		if v, _ := n.Props[StixID].(string); v == stixID { // ext: cti-import
			return n // ext: cti-import
		} // ext: cti-import
	} // ext: cti-import
	return nil // ext: cti-import
} // ext: cti-import

// findGraphID scans all nodes for stixID and returns its graph ID, or "". // ext: cti-import
func findGraphID(s storage.StorageEngine, stixID string) string { // ext: cti-import
	for _, n := range s.GetAllNodes() { // ext: cti-import
		if v, _ := n.Props[StixID].(string); v == stixID { // ext: cti-import
			return n.ID // ext: cti-import
		} // ext: cti-import
	} // ext: cti-import
	return "" // ext: cti-import
} // ext: cti-import

// mergeProps overlays new on old without mutating either input. // ext: cti-import
func mergeProps(old, new map[string]any) map[string]any { // ext: cti-import
	out := make(map[string]any, len(old)+len(new)) // ext: cti-import
	for k, v := range old {                        // ext: cti-import
		out[k] = v // ext: cti-import
	} // ext: cti-import
	for k, v := range new { // ext: cti-import
		out[k] = v // ext: cti-import
	} // ext: cti-import
	return out // ext: cti-import
} // ext: cti-import
