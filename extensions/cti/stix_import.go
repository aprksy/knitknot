// ext: cti-import — STIX 2.1 bundle importer (stream, stage to disk, then upsert).
package cti

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
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
	StixID string         `json:"stixId"` // ext: cti-import
	Label  string         `json:"label"`  // ext: cti-import
	Props  map[string]any `json:"props"`  // ext: cti-import
} // ext: cti-import

type stagedRel struct { // ext: cti-import
	StixID  string         `json:"stixId"`  // ext: cti-import
	Kind    string         `json:"kind"`    // ext: cti-import
	FromRef string         `json:"fromRef"` // ext: cti-import
	ToRef   string         `json:"toRef"`   // ext: cti-import
	Props   map[string]any `json:"props"`   // ext: cti-import
} // ext: cti-import

// Import reads a STIX 2.1 bundle and upserts its objects as nodes and edges.
//
// perf: the bundle is streamed one object at a time and staged to scratch
// files; nothing proportional to bundle size is kept in memory. The previous
// `stix2.FromJSON` path materialised the whole bundle as a Collection plus
// `[]stagedNode`/`[]stagedRel` copies, which OOM-killed a 52MB bundle at
// ~15GB anon-rss. Peak memory is now one object plus the id/label indexes.
// ext: cti-import
func (im *STIXImporter) Import(ctx context.Context, ic extension.ImportContext, s storage.StorageEngine, verbs *types.VerbRegistry, r io.Reader) error { // ext: cti-import
	_ = verbs // ext: cti-import

	nodes, err := os.CreateTemp("", "knitknot-stix-nodes-*.jsonl") // ext: cti-import
	if err != nil {                                                // ext: cti-import
		return fmt.Errorf("cti: staging file: %w", err) // ext: cti-import
	}
	nodesPath := nodes.Name() // ext: cti-import
	defer func() {            // ext: cti-import
		_ = nodes.Close() // ext: cti-import
		_ = os.Remove(nodesPath)
	}()
	rels, err := os.CreateTemp("", "knitknot-stix-rels-*.jsonl") // ext: cti-import
	if err != nil {                                              // ext: cti-import
		return fmt.Errorf("cti: staging file: %w", err) // ext: cti-import
	}
	relsPath := rels.Name() // ext: cti-import
	defer func() {          // ext: cti-import
		_ = rels.Close() // ext: cti-import
		_ = os.Remove(relsPath)
	}()

	// Pass 1: stream, validate, stage. No storage mutation happens here, so a
	// malformed bundle still leaves the graph untouched.
	staged, nNodes, nRels, err := im.stage(ctx, ic, r, bufio.NewWriter(nodes), bufio.NewWriter(rels)) // ext: cti-import
	if err != nil {                                                                                   // ext: cti-import
		return err // ext: cti-import
	} // ext: cti-import
	if nNodes == 0 && nRels == 0 { // ext: cti-import
		return fmt.Errorf("cti: bundle has no objects") // ext: cti-import
	} // ext: cti-import

	// Still pass 1: every relationship endpoint must resolve against the
	// staged set or pre-existing storage before any mutation happens.
	if err := im.checkEndpoints(s, staged, rels); err != nil { // ext: cti-import
		return err // ext: cti-import
	} // ext: cti-import

	// Pass 2: upsert nodes, then relationships, both read back as streams.
	pass2 := func() error { // ext: cti-import
		if err := im.upsertNodes(ctx, ic, s, nodes); err != nil { // ext: cti-import
			return err // ext: cti-import
		} // ext: cti-import
		return im.upsertRels(ctx, ic, s, rels) // ext: cti-import
	} // ext: cti-import
	if bs, ok := s.(batchStore); ok { // ext: cti-import
		if err := bs.Batch(pass2); err != nil { // ext: cti-import
			return err // ext: cti-import
		} // ext: cti-import
	} else if err := pass2(); err != nil { // ext: cti-import
		return err // ext: cti-import
	} // ext: cti-import

	if ic.MaterializeTactics { // ext: cti-tactics — opt-in; default import returns above unchanged
		if _, _, err := materializeTactics(ctx, s, ic); err != nil { // ext: cti-tactics
			return fmt.Errorf("cti: materialize tactics: %w", err) // ext: cti-tactics
		} // ext: cti-tactics
	} // ext: cti-tactics
	return nil // ext: cti-import
} // ext: cti-import

// stage streams the bundle, validates each object and writes nodes and
// relationships to their own staging files. It returns the set of staged
// STIX ids plus the two counts.
func (im *STIXImporter) stage(ctx context.Context, ic extension.ImportContext, r io.Reader, nodeW, relW *bufio.Writer) (map[string]struct{}, int, int, error) { // ext: cti-import
	staged := make(map[string]struct{}) // ext: cti-import
	nNodes, nRels := 0, 0               // ext: cti-import

	dec := json.NewDecoder(bufio.NewReader(r)) // ext: cti-import
	tok, err := dec.Token()                    // ext: cti-import
	if err != nil {                            // ext: cti-import
		return nil, 0, 0, fmt.Errorf("cti: parse STIX bundle: %w", err) // ext: cti-import
	} // ext: cti-import

	switch d := tok.(type) { // ext: cti-import
	case json.Delim: // ext: cti-import
		switch d { // ext: cti-import
		case '[': // ext: cti-import — a bare array of objects, the other shape FromJSON accepts
			if err := im.stageArray(ctx, ic, dec, nodeW, relW, staged, &nNodes, &nRels); err != nil { // ext: cti-import
				return nil, 0, 0, err // ext: cti-import
			} // ext: cti-import
		case '{': // ext: cti-import — a bundle object
			if err := im.stageBundle(ctx, ic, dec, nodeW, relW, staged, &nNodes, &nRels); err != nil { // ext: cti-import
				return nil, 0, 0, err // ext: cti-import
			} // ext: cti-import
		default: // ext: cti-import
			return nil, 0, 0, fmt.Errorf("cti: parse STIX bundle: unexpected %q", d) // ext: cti-import
		} // ext: cti-import
	default: // ext: cti-import
		return nil, 0, 0, fmt.Errorf("cti: parse STIX bundle: expected an object or array") // ext: cti-import
	} // ext: cti-import

	if err := nodeW.Flush(); err != nil { // ext: cti-import
		return nil, 0, 0, fmt.Errorf("cti: staging flush: %w", err) // ext: cti-import
	} // ext: cti-import
	if err := relW.Flush(); err != nil { // ext: cti-import
		return nil, 0, 0, fmt.Errorf("cti: staging flush: %w", err) // ext: cti-import
	} // ext: cti-import
	return staged, nNodes, nRels, nil // ext: cti-import
} // ext: cti-import

// stageBundle walks the top-level members of a bundle object.
func (im *STIXImporter) stageBundle(ctx context.Context, ic extension.ImportContext, dec *json.Decoder, nodeW, relW *bufio.Writer, staged map[string]struct{}, nNodes, nRels *int) error { // ext: cti-import
	for dec.More() { // ext: cti-import
		keyTok, err := dec.Token() // ext: cti-import
		if err != nil {            // ext: cti-import
			return fmt.Errorf("cti: parse STIX bundle: %w", err) // ext: cti-import
		} // ext: cti-import
		key, _ := keyTok.(string) // ext: cti-import
		if key != "objects" {     // ext: cti-import
			var skip json.RawMessage                  // ext: cti-import
			if err := dec.Decode(&skip); err != nil { // ext: cti-import
				return fmt.Errorf("cti: parse STIX bundle: %w", err) // ext: cti-import
			} // ext: cti-import
			continue // ext: cti-import
		} // ext: cti-import
		if err := im.stageArray(ctx, ic, dec, nodeW, relW, staged, nNodes, nRels); err != nil { // ext: cti-import
			return err // ext: cti-import
		} // ext: cti-import
	} // ext: cti-import
	// consume the closing '}'
	if _, err := dec.Token(); err != nil { // ext: cti-import
		return fmt.Errorf("cti: parse STIX bundle: %w", err) // ext: cti-import
	} // ext: cti-import
	return nil // ext: cti-import
} // ext: cti-import

// stageArray streams one JSON array of objects (the bundle's "objects" member
// or a top-level array).
func (im *STIXImporter) stageArray(ctx context.Context, ic extension.ImportContext, dec *json.Decoder, nodeW, relW *bufio.Writer, staged map[string]struct{}, nNodes, nRels *int) error { // ext: cti-import
	tok, err := dec.Token() // ext: cti-import
	if err != nil {         // ext: cti-import
		return fmt.Errorf("cti: parse STIX bundle: %w", err) // ext: cti-import
	} // ext: cti-import
	if d, ok := tok.(json.Delim); !ok || d != '[' { // ext: cti-import
		return fmt.Errorf("cti: parse STIX bundle: \"objects\" must be an array") // ext: cti-import
	} // ext: cti-import

	for i := 0; dec.More(); i++ { // ext: cti-import
		if i%64 == 0 { // ext: cti-import
			if err := ctx.Err(); err != nil { // ext: cti-import
				return err // ext: cti-import
			} // ext: cti-import
		} // ext: cti-import
		var raw json.RawMessage                  // ext: cti-import
		if err := dec.Decode(&raw); err != nil { // ext: cti-import
			return fmt.Errorf("cti: parse STIX bundle: %w", err) // ext: cti-import
		} // ext: cti-import

		// One object at a time through the library's own parser, so type
		// dispatch and custom-property handling stay exactly the library's.
		obj, err := parseOne(raw) // ext: cti-import
		if err != nil {           // ext: cti-import
			return err // ext: cti-import
		} // ext: cti-import
		if obj == nil { // ext: cti-import
			continue // ext: cti-import — no "type": the library skips these too // ext: cti-import
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
			props, err := graphProps(obj, ic.Source) // ext: cti-import // ext: source-feed
			if err != nil {                          // ext: cti-import
				return fmt.Errorf("cti: relationship %q: %w", stixID, err) // ext: cti-import
			} // ext: cti-import
			if err := writeStaged(relW, stagedRel{StixID: stixID, Kind: string(rel.RelationshipType), FromRef: string(rel.Source), ToRef: string(rel.Target), Props: props}); err != nil { // ext: cti-import
				return err // ext: cti-import
			} // ext: cti-import
			*nRels++ // ext: cti-import
			continue // ext: cti-import
		} // ext: cti-import

		if ind, ok := obj.(*stix2.Indicator); ok { // ext: cti-import
			if ind.Pattern == "" || ind.PatternType == "" || ind.ValidFrom == nil { // ext: cti-import
				return fmt.Errorf("cti: indicator %q missing required pattern, pattern_type, or valid_from", stixID) // ext: cti-import
			} // ext: cti-import
		} // ext: cti-import

		props, err := graphProps(obj, ic.Source) // ext: cti-import // ext: source-feed
		if err != nil {                          // ext: cti-import
			return fmt.Errorf("cti: object %q: %w", stixID, err) // ext: cti-import
		} // ext: cti-import
		// ext: cti-import — label is the STIX type verbatim; supported SDOs match vocabulary consts, anything else (SCOs, x-custom) is preserved as-is.
		if err := writeStaged(nodeW, stagedNode{StixID: stixID, Label: string(obj.GetType()), Props: props}); err != nil { // ext: cti-import
			return err // ext: cti-import
		} // ext: cti-import
		staged[stixID] = struct{}{} // ext: cti-import
		*nNodes++                   // ext: cti-import
	} // ext: cti-import

	// consume the closing ']'
	if _, err := dec.Token(); err != nil { // ext: cti-import
		return fmt.Errorf("cti: parse STIX bundle: %w", err) // ext: cti-import
	} // ext: cti-import
	return nil // ext: cti-import
} // ext: cti-import

// parseOne decodes a single raw STIX object. The library exposes only a
// whole-input FromJSON, so the object is wrapped in a one-element array —
// its smallest accepted streaming shape — and released as soon as the caller
// stages it.
func parseOne(raw json.RawMessage) (stix2.STIXObject, error) { // ext: cti-import
	buf := make([]byte, 0, len(raw)+2)     // ext: cti-import
	buf = append(buf, '[')                 // ext: cti-import
	buf = append(buf, raw...)              // ext: cti-import
	buf = append(buf, ']')                 // ext: cti-import
	collection, err := stix2.FromJSON(buf) // ext: cti-import
	if err != nil {                        // ext: cti-import
		return nil, fmt.Errorf("cti: parse STIX object: %w", err) // ext: cti-import
	} // ext: cti-import
	objs := collection.AllObjects() // ext: cti-import
	if len(objs) == 0 {             // ext: cti-import
		return nil, nil // ext: cti-import
	} // ext: cti-import
	return objs[0], nil // ext: cti-import
} // ext: cti-import

// writeStaged appends one staged object to a staging file as a JSON line.
func writeStaged(w *bufio.Writer, v any) error { // ext: cti-import
	line, err := json.Marshal(v) // ext: cti-import
	if err != nil {              // ext: cti-import
		return fmt.Errorf("cti: stage object: %w", err) // ext: cti-import
	} // ext: cti-import
	line = append(line, '\n')                // ext: cti-import
	if _, err := w.Write(line); err != nil { // ext: cti-import
		return fmt.Errorf("cti: staging write: %w", err) // ext: cti-import
	} // ext: cti-import
	return nil // ext: cti-import
} // ext: cti-import

// checkEndpoints re-reads the relationship staging file and verifies every
// endpoint resolves against the staged set or storage. It runs before any
// mutation, so a dangling reference leaves the graph untouched.
func (im *STIXImporter) checkEndpoints(s storage.StorageEngine, staged map[string]struct{}, rels *os.File) error { // ext: cti-import
	if _, err := rels.Seek(0, io.SeekStart); err != nil { // ext: cti-import
		return fmt.Errorf("cti: staging rewind: %w", err) // ext: cti-import
	} // ext: cti-import
	sc := newScanner(rels) // ext: cti-import
	for sc.Scan() {        // ext: cti-import
		var rel stagedRel                                        // ext: cti-import
		if err := json.Unmarshal(sc.Bytes(), &rel); err != nil { // ext: cti-import
			return fmt.Errorf("cti: read staged relationship: %w", err) // ext: cti-import
		} // ext: cti-import
		for _, ref := range []string{rel.FromRef, rel.ToRef} { // ext: cti-import
			if _, ok := staged[ref]; ok { // ext: cti-import
				continue // ext: cti-import
			} // ext: cti-import
			if findGraphID(s, ref) == "" { // ext: cti-import
				return fmt.Errorf("cti: relationship %q references unknown object %q", rel.StixID, ref) // ext: cti-import
			} // ext: cti-import
		} // ext: cti-import
	} // ext: cti-import
	if err := sc.Err(); err != nil { // ext: cti-import
		return fmt.Errorf("cti: read staging: %w", err) // ext: cti-import
	} // ext: cti-import
	if _, err := rels.Seek(0, io.SeekStart); err != nil { // ext: cti-import
		return fmt.Errorf("cti: staging rewind: %w", err) // ext: cti-import
	} // ext: cti-import
	return nil // ext: cti-import
} // ext: cti-import

// upsertNodes streams the node staging file, upserting each node.
func (im *STIXImporter) upsertNodes(ctx context.Context, ic extension.ImportContext, s storage.StorageEngine, nodes *os.File) error { // ext: cti-import
	if _, err := nodes.Seek(0, io.SeekStart); err != nil { // ext: cti-import
		return fmt.Errorf("cti: staging rewind: %w", err) // ext: cti-import
	} // ext: cti-import
	sc := newScanner(nodes) // ext: cti-import
	for sc.Scan() {         // ext: cti-import
		var n stagedNode                                       // ext: cti-import
		if err := json.Unmarshal(sc.Bytes(), &n); err != nil { // ext: cti-import
			return fmt.Errorf("cti: read staged node: %w", err) // ext: cti-import
		} // ext: cti-import
		if err := ctx.Err(); err != nil { // ext: cti-import
			return err // ext: cti-import
		} // ext: cti-import
		if err := im.upsertNode(ctx, ic, s, n); err != nil { // ext: cti-import
			return err // ext: cti-import
		} // ext: cti-import
	} // ext: cti-import
	if err := sc.Err(); err != nil { // ext: cti-import
		return fmt.Errorf("cti: read staging: %w", err) // ext: cti-import
	} // ext: cti-import
	return nil // ext: cti-import
} // ext: cti-import

// upsertRels streams the relationship staging file and upserts each edge.
func (im *STIXImporter) upsertRels(ctx context.Context, ic extension.ImportContext, s storage.StorageEngine, rels *os.File) error { // ext: cti-import
	if _, err := rels.Seek(0, io.SeekStart); err != nil { // ext: cti-import
		return fmt.Errorf("cti: staging rewind: %w", err) // ext: cti-import
	} // ext: cti-import
	sc := newScanner(rels) // ext: cti-import
	skipped := 0           // ext: cti-import
	for sc.Scan() {        // ext: cti-import
		var rel stagedRel                                        // ext: cti-import
		if err := json.Unmarshal(sc.Bytes(), &rel); err != nil { // ext: cti-import
			return fmt.Errorf("cti: read staged relationship: %w", err) // ext: cti-import
		} // ext: cti-import
		if err := ctx.Err(); err != nil { // ext: cti-import
			return err // ext: cti-import
		} // ext: cti-import
		stale, err := im.upsertRel(ctx, ic, s, rel) // ext: cti-import
		if err != nil {                             // ext: cti-import
			return err // ext: cti-import
		} // ext: cti-import
		if stale { // ext: cti-import
			skipped++ // ext: cti-import
		} // ext: cti-import
	} // ext: cti-import
	if err := sc.Err(); err != nil { // ext: cti-import
		return fmt.Errorf("cti: read staging: %w", err) // ext: cti-import
	} // ext: cti-import
	if skipped > 0 { // ext: cti-import
		fmt.Fprintf(os.Stderr, "cti: skipped %d stale object(s) older than stored snapshots\n", skipped) // ext: cti-import
	} // ext: cti-import
	return nil // ext: cti-import
} // ext: cti-import

// newScanner returns a line scanner with a payload cap large enough for a
// single STIX object's props (the default 64KB is too small for some SDOs).
func newScanner(r io.Reader) *bufio.Scanner { // ext: cti-import
	sc := bufio.NewScanner(r) // ext: cti-import
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	return sc // ext: cti-import
} // ext: cti-import

// upsertNode upserts one staged node, merging props on an existing stix_id.
func (im *STIXImporter) upsertNode(ctx context.Context, ic extension.ImportContext, s storage.StorageEngine, n stagedNode) error { // ext: cti-import
	_ = ctx                              // ext: cti-import
	existing := findGraphID(s, n.StixID) // ext: cti-import
	if existing != "" {                  // ext: cti-import
		if node, ok := s.GetNode(existing); ok { // ext: cti-import
			if isStale(node.Props, n.Props) { // ext: cti-import
				return nil // ext: cti-import
			} // ext: cti-import
			if err := updateNodeSrc(s, existing, mergeProps(node.Props, n.Props), ic.Source, ic.Transaction); err != nil { // ext: cti-import
				return fmt.Errorf("cti: update node %q: %w", n.StixID, err) // ext: cti-import
			} // ext: cti-import
			return nil // ext: cti-import
		} // ext: cti-import
	} // ext: cti-import
	if _, err := addNodeSrc(s, n.Label, n.Props, ic.Source, ic.Transaction); err != nil { // ext: cti-import
		return fmt.Errorf("cti: add node %q: %w", n.StixID, err) // ext: cti-import
	} // ext: cti-import
	return nil // ext: cti-import
} // ext: cti-import

// upsertRel upserts one staged relationship. It reports whether the incoming
// object was stale (and therefore skipped).
func (im *STIXImporter) upsertRel(ctx context.Context, ic extension.ImportContext, s storage.StorageEngine, rel stagedRel) (bool, error) { // ext: cti-import
	_ = ctx                                // ext: cti-import
	from := resolveGraphID(s, rel.FromRef) // ext: cti-import
	to := resolveGraphID(s, rel.ToRef)     // ext: cti-import
	if from == "" || to == "" {            // ext: cti-import
		return false, fmt.Errorf("cti: relationship %q has unresolvable endpoint", rel.StixID) // ext: cti-import
	} // ext: cti-import
	if found := findEdge(s, from, to, rel.Kind); found != nil { // ext: cti-import
		if isStale(found.Props, rel.Props) { // ext: cti-import
			return true, nil // ext: cti-import
		} // ext: cti-import
		if err := updateEdgeSrc(s, found.ID, mergeProps(found.Props, rel.Props), ic.Source, ic.Transaction); err != nil { // ext: cti-import
			return false, fmt.Errorf("cti: update edge %q: %w", rel.StixID, err) // ext: cti-import
		} // ext: cti-import
		return false, nil // ext: cti-import
	} // ext: cti-import
	if err := addEdgeSrc(s, from, to, rel.Kind, rel.Props, ic.Source, ic.Transaction); err != nil { // ext: cti-import
		return false, fmt.Errorf("cti: add edge %q: %w", rel.StixID, err) // ext: cti-import
	} // ext: cti-import
	return false, nil // ext: cti-import
} // ext: cti-import

// resolveGraphID maps a staged STIX id to its graph id, falling back to a
// storage scan for objects that pre-date this import.
func resolveGraphID(s storage.StorageEngine, stixID string) string { // ext: cti-import
	return findGraphID(s, stixID) // ext: cti-import
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

// findEdge returns the stored edge for (from, to, kind), or nil. // ext: cti-import
func findEdge(s storage.StorageEngine, from, to, kind string) *types.Edge { // ext: cti-import
	if e, ok := s.GetEdge(from + "->" + to + "@" + kind); ok { // ext: cti-import
		return e // ext: cti-import
	} // ext: cti-import
	for _, e := range s.GetEdgesFrom(from) { // ext: cti-import — non-deterministic backend fallback
		if e.To == to && e.Kind == kind { // ext: cti-import
			return e // ext: cti-import
		} // ext: cti-import
	} // ext: cti-import
	return nil // ext: cti-import
} // ext: cti-import

// isStale reports whether newProps predates oldProps by STIX `modified`.
// Missing or unparseable timestamps on either side mean "can't tell" —
// not stale, so the update proceeds as before. Equal timestamps proceed.
func isStale(oldProps, newProps map[string]any) bool { // ext: cti-import
	oldT, ok := stixTime(oldProps["modified"]) // ext: cti-import
	if !ok {                                   // ext: cti-import
		return false // ext: cti-import
	} // ext: cti-import
	newT, ok := stixTime(newProps["modified"]) // ext: cti-import
	if !ok {                                   // ext: cti-import
		return false // ext: cti-import
	} // ext: cti-import
	return newT.Before(oldT) // ext: cti-import
} // ext: cti-import

func stixTime(v any) (time.Time, bool) { // ext: cti-import
	s, ok := v.(string) // ext: cti-import
	if !ok {            // ext: cti-import
		return time.Time{}, false // ext: cti-import
	} // ext: cti-import
	t, err := time.Parse(time.RFC3339, s) // ext: cti-import
	if err != nil {                       // ext: cti-import
		return time.Time{}, false // ext: cti-import
	} // ext: cti-import
	return t, true // ext: cti-import
} // ext: cti-import

// graphProps flattens a STIX object to generic node/edge props via its JSON
// form, so known fields and raw custom properties survive without a
// per-type field list.
func graphProps(obj stix2.STIXObject, source string) (map[string]any, error) { // ext: cti-import // ext: source-feed
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
	props[SourceFeed] = source                                // ext: cti-import // ext: source-feed
	props[ImportedAt] = time.Now().UTC().Format(time.RFC3339) // ext: cti-import
	return props, nil                                         // ext: cti-import
} // ext: cti-import

type batchStore interface { // ext: cti-import
	Batch(fn func() error) error // ext: cti-import
} // ext: cti-import

// metaStore is the optional versioning write path carrying source/transaction.
// Backends without it fall back to plain StorageEngine writes (Source lost).
type metaStore interface { // ext: source-feed
	AddNodeWithMeta(label string, props map[string]any, source, transaction string) (string, error) // ext: source-feed
	AddEdgeWithMeta(from, to, kind string, props map[string]any, source, transaction string) error  // ext: source-feed
	UpdateNodeWithMeta(id string, props map[string]any, source, transaction string) error           // ext: source-feed
	UpdateEdgeWithMeta(id string, props map[string]any, source, transaction string) error           // ext: source-feed
} // ext: source-feed

func addNodeSrc(s storage.StorageEngine, label string, props map[string]any, source, tx string) (string, error) { // ext: source-feed
	if ms, ok := s.(metaStore); ok { // ext: source-feed
		return ms.AddNodeWithMeta(label, props, source, tx) // ext: source-feed
	} // ext: source-feed
	return s.AddNode(label, props) // ext: source-feed
} // ext: source-feed

func addEdgeSrc(s storage.StorageEngine, from, to, kind string, props map[string]any, source, tx string) error { // ext: source-feed
	if ms, ok := s.(metaStore); ok { // ext: source-feed
		return ms.AddEdgeWithMeta(from, to, kind, props, source, tx) // ext: source-feed
	} // ext: source-feed
	return s.AddEdge(from, to, kind, props) // ext: source-feed
} // ext: source-feed

func updateNodeSrc(s storage.StorageEngine, id string, props map[string]any, source, tx string) error { // ext: source-feed
	if ms, ok := s.(metaStore); ok { // ext: source-feed
		return ms.UpdateNodeWithMeta(id, props, source, tx) // ext: source-feed
	} // ext: source-feed
	return s.UpdateNode(id, props) // ext: source-feed
} // ext: source-feed

func updateEdgeSrc(s storage.StorageEngine, id string, props map[string]any, source, tx string) error { // ext: source-feed
	if ms, ok := s.(metaStore); ok { // ext: source-feed
		return ms.UpdateEdgeWithMeta(id, props, source, tx) // ext: source-feed
	} // ext: source-feed
	return s.UpdateEdge(id, props) // ext: source-feed
} // ext: source-feed

// mergeProps overlays newer values onto a base map. // ext: cti-import
func mergeProps(base, newer map[string]any) map[string]any { // ext: cti-import
	if base == nil { // ext: cti-import
		base = make(map[string]any, len(newer)) // ext: cti-import
	} // ext: cti-import
	for k, v := range newer { // ext: cti-import
		base[k] = v // ext: cti-import
	} // ext: cti-import
	return base // ext: cti-import
} // ext: cti-import

// unused keeps bufio imported for the scanner helpers above.
var _ = bufio.NewReader // ext: cti-import
