package cmd

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/aprksy/knitknot/pkg/ports/storage"
	"github.com/aprksy/knitknot/pkg/ports/types"
)

// store: sync — ADR 0001 merge: additive upsert of every node/edge from
// --from into --to using the CTI STIXImporter pattern (stable-ID lookup +
// source-wins property merge), --prune for destructive propagation.
// Sync holds one engine at a time, never both locks simultaneously
// (single-writer rule); mutations group under one transaction ID.
func newSyncCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "sync",
		Short: "Sync graph data between storage backends",
		RunE: func(cmd *cobra.Command, args []string) error {
			fromURI, _ := cmd.Flags().GetString("from")
			if fromURI == "" {
				return fmt.Errorf("missing required --from flag (want --from <store-uri>)")
			}
			toURI, _ := cmd.Flags().GetString("to")
			if toURI == "" {
				return fmt.Errorf("missing required --to flag (want --to <store-uri>)")
			}
			prune, _ := cmd.Flags().GetBool("prune")
			return runSync(fromURI, toURI, prune)
		},
	}
	c.Flags().String("from", "", "Source store URI (e.g. gob:file.gob)")
	c.Flags().String("to", "", "Destination store URI (e.g. gob:other.gob)")
	c.Flags().Bool("prune", false, "Delete nodes/edges in --to absent from --from")
	c.SilenceUsage = true
	return c
}

// store: sync — bare paths default to gob: (matching -f behavior); URIs keep
// their scheme so unregistered schemes still name correctly in errors.
func parseSyncURI(uri string) (scheme, path string) {
	if i := strings.Index(uri, ":"); i > 0 {
		return uri[:i], uri[i+1:]
	}
	return "gob", uri
}

func runSync(fromURI, toURI string, prune bool) error {
	fromScheme, fromPath := parseSyncURI(fromURI)
	toScheme, toPath := parseSyncURI(toURI)
	if fromScheme != "gob" || toScheme != "gob" {
		return fmt.Errorf("sync not implemented for %s -> %s yet", fromScheme, toScheme)
	}
	ensureDefaultStore()
	sel := storage.Default()
	// store: sync — resolve through the selector (validates registration);
	// gobBackend.Open(path) is LoadGraph(path).Storage(), so load via
	// LoadGraph to also retain the verb registries Open drops.
	if _, err := sel.Resolve(fromScheme + ":" + fromPath); err != nil {
		return err
	}
	if _, err := sel.Resolve(toScheme + ":" + toPath); err != nil {
		return err
	}
	fromEngine, err := LoadGraph(fromPath)
	if err != nil {
		return err
	}
	toEngine, err := LoadGraph(toPath)
	if err != nil {
		return err
	}
	fromSE, toSE := fromEngine.Storage(), toEngine.Storage()

	// Pass 1: snapshot source, validate every edge endpoint against the
	// source's full node set before any mutation (mirrors STIXImporter).
	srcNodes := fromSE.GetAllNodes()
	srcEdges := fromSE.GetAllEdges()
	srcIDs := make(map[string]struct{}, len(srcNodes))
	for _, n := range srcNodes {
		srcIDs[n.ID] = struct{}{}
	}
	for _, e := range srcEdges {
		if _, ok := srcIDs[e.From]; !ok {
			return fmt.Errorf("sync: edge %q references unknown source node %q; aborting before any mutation", e.ID, e.From)
		}
		if _, ok := srcIDs[e.To]; !ok {
			return fmt.Errorf("sync: edge %q references unknown source node %q; aborting before any mutation", e.ID, e.To)
		}
	}

	// store: sync — one transaction groups all mutations (CTI importer style).
	tx := fmt.Sprintf("sync-%s", time.Now().UTC().Format(time.RFC3339Nano))

	// Pass 2: upsert nodes by stable ID (node.ID is the storage-assigned
	// canonical key); new nodes get fresh IDs tracked in idMap so edges
	// remap onto them. Source props win on conflict.
	idMap := make(map[string]string, len(srcNodes))
	for _, sn := range srcNodes {
		if existing, ok := toSE.GetNode(sn.ID); ok {
			if merged := syncMergeProps(existing.Props, sn.Props); !reflect.DeepEqual(existing.Props, merged) {
				if err := updateNodeSync(toSE, existing.ID, merged, tx); err != nil {
					return fmt.Errorf("sync: update node %q: %w", sn.ID, err)
				}
			}
			idMap[sn.ID] = existing.ID
			continue
		}
		newID, err := addNodeSync(toSE, sn.Label, sn.Props, tx)
		if err != nil {
			return fmt.Errorf("sync: add node %q: %w", sn.ID, err)
		}
		idMap[sn.ID] = newID
	}
	// Upsert edges by (from, to, kind) triple — the edge's stable identity
	// (edge.ID derives from it) — same GetEdgesFrom scan as STIXImporter.
	for _, se := range srcEdges {
		from, to := idMap[se.From], idMap[se.To]
		var found *types.Edge
		for _, e := range toSE.GetEdgesFrom(from) {
			if e.To == to && e.Kind == se.Kind {
				found = e
				break
			}
		}
		if found != nil {
			if merged := syncMergeProps(found.Props, se.Props); !reflect.DeepEqual(found.Props, merged) {
				if err := updateEdgeSync(toSE, found.ID, merged, tx); err != nil {
					return fmt.Errorf("sync: update edge %q: %w", se.ID, err)
				}
			}
			continue
		}
		if err := addEdgeSync(toSE, from, to, se.Kind, se.Props, tx); err != nil {
			return fmt.Errorf("sync: add edge %q: %w", se.ID, err)
		}
	}
	// Verbs merge by name; existing --to verbs survive unless overridden.
	for name, def := range fromEngine.Verbs().All() {
		toEngine.RegisterVerb(name, def)
	}
	if prune {
		wantEdges := make(map[string]struct{}, len(srcEdges))
		for _, se := range srcEdges {
			wantEdges[idMap[se.From]+"\x00"+idMap[se.To]+"\x00"+se.Kind] = struct{}{}
		}
		for _, e := range toSE.GetAllEdges() {
			if _, ok := wantEdges[e.From+"\x00"+e.To+"\x00"+e.Kind]; !ok {
				if err := deleteEdgeSync(toSE, e.From, e.To, e.Kind, tx); err != nil {
					return fmt.Errorf("sync: prune edge %q: %w", e.ID, err)
				}
			}
		}
		keep := make(map[string]struct{}, len(idMap))
		for _, id := range idMap {
			keep[id] = struct{}{}
		}
		for _, n := range toSE.GetAllNodes() {
			if _, ok := keep[n.ID]; !ok {
				if err := deleteNodeSync(toSE, n.ID, tx); err != nil {
					return fmt.Errorf("sync: prune node %q: %w", n.ID, err)
				}
			}
		}
	}
	return SaveGraph(toEngine, toPath)
}

// store: sync — optional versioned write path carrying source/transaction;
// backends without it fall back to plain StorageEngine writes.
type syncMetaStore interface {
	AddNodeWithMeta(label string, props map[string]any, source, transaction string) (string, error)
	AddEdgeWithMeta(from, to, kind string, props map[string]any, source, transaction string) error
	UpdateNodeWithMeta(id string, props map[string]any, source, transaction string) error
	UpdateEdgeWithMeta(id string, props map[string]any, source, transaction string) error
	DeleteNodeWithMeta(id, source, transaction string) error
	DeleteEdgeWithMeta(from, to, kind, source, transaction string) error
}

// store: sync — provenance source recorded on every synced snapshot.
const syncSource = "sync"

func addNodeSync(s storage.StorageEngine, label string, props map[string]any, tx string) (string, error) {
	if ms, ok := s.(syncMetaStore); ok {
		return ms.AddNodeWithMeta(label, props, syncSource, tx)
	}
	return s.AddNode(label, props)
}

func addEdgeSync(s storage.StorageEngine, from, to, kind string, props map[string]any, tx string) error {
	if ms, ok := s.(syncMetaStore); ok {
		return ms.AddEdgeWithMeta(from, to, kind, props, syncSource, tx)
	}
	return s.AddEdge(from, to, kind, props)
}

func updateNodeSync(s storage.StorageEngine, id string, props map[string]any, tx string) error {
	if ms, ok := s.(syncMetaStore); ok {
		return ms.UpdateNodeWithMeta(id, props, syncSource, tx)
	}
	return s.UpdateNode(id, props)
}

func updateEdgeSync(s storage.StorageEngine, id string, props map[string]any, tx string) error {
	if ms, ok := s.(syncMetaStore); ok {
		return ms.UpdateEdgeWithMeta(id, props, syncSource, tx)
	}
	return s.UpdateEdge(id, props)
}

func deleteNodeSync(s storage.StorageEngine, id, tx string) error {
	if ms, ok := s.(syncMetaStore); ok {
		return ms.DeleteNodeWithMeta(id, syncSource, tx)
	}
	return s.DeleteNode(id)
}

func deleteEdgeSync(s storage.StorageEngine, from, to, kind, tx string) error {
	if ms, ok := s.(syncMetaStore); ok {
		return ms.DeleteEdgeWithMeta(from, to, kind, syncSource, tx)
	}
	return s.DeleteEdge(from, to, kind)
}

// store: sync — source-wins overlay, same rule as the CTI importer: incoming
// props override, existing keys survive, neither input mutated.
func syncMergeProps(old, new map[string]any) map[string]any {
	out := make(map[string]any, len(old)+len(new))
	for k, v := range old {
		out[k] = v
	}
	for k, v := range new {
		out[k] = v
	}
	return out
}

func init() {
	RootCmd.AddCommand(newSyncCmd())
}
