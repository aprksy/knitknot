# ADR 0012 — Focused (on-demand) in-memory cache over a disk backend

- **Status:** proposed
- **Date:** 2026-11-26
- **Deciders:** aprksy (owner)
- **Lane:** `perf/focused-cache` (off `dev`), measured on `sxi-work`

## Context

knitknot's default backend (`pkg/storage/inmem`, via the `gob` scheme) holds
the whole graph resident. That is right for the analyst / REPL niche, but it
stops being true at STIX scale. Measured on `sxi-work` (16 GB, WSL2, 4 GB
swap) on 2026-11-26 with the 52 MB
`extensions/cti/testdata/enterprise-attack.json` bundle:

| Path | Result |
|---|---|
| `inmem` (probe, `cti.STIXImporter` direct) | OOM-killed — `anon-rss:14766660kB` |
| `--store bolt:` (full CLI import) | OOM-killed — `anon-rss:14883800kB` |
| `--store bolt:` (repeat) | OOM-killed — `anon-rss:14990560kB` |

`dmesg`: `Out of memory: Killed process … (knitknot) total-vm:20859552kB
anon-rss:14883800kB`. Peak RSS before the kill was 14.6 GB on the bolt path.
So the 7 GB figure the owner reported is not a ceiling — on this box the
import does not complete at all.

The cost is structural, not a tuning problem. Three multiplicative terms:

1. **Props are stored twice.** `extensions/cti.graphProps` flattens each STIX
   object with `json.Marshal` → `json.Unmarshal` into `map[string]any`, and
   `AddNodeWithMeta` stores that map both on `Node.Props` and again on
   `Node.History[0].Props`.
2. **Go map overhead + a second staging copy.** A 30-key `map[string]any`
   costs ~1.5–2 KB of buckets; the importer also keeps `[]stagedNode` /
   `[]stagedRel` alive for the whole of pass 1 before pass 2 writes.
3. **Bolt does not help yet.** `bolt.GetNode` decodes the node's gob —
   including its inline `History` — on every point read, so the disk backend
   pays the same per-element double copy in memory, and `GetAllNodes` is a
   full cursor scan.

45 MB → 14.6 GB is ~330×, which is what "two copies of a map-heavy object
graph, plus a staging copy" costs.

The engine is already shaped for a lazy backend:

- `pkg/ports/storage.Backend`/`Selector` select by URI scheme (`--store`);
  `cmd/store_bolt.go` already registers `bolt`.
- `pkg/storage/bolt` does not preload; `GetNode` is a point get.
- The query engine only asks for `GetNodesByLabel`, `GetEdgesFrom/To`,
  `GetEdgesByKind`, `GetNode(id)`. `GetAllNodes`/`GetAllEdges` are used only
  by exporters, `sync`, and the STIX/memory importers.
- ADR 0004 already ships bounded BFS (`.Reach(rel, dir, maxDepth)`,
  `.Follow`), so "N hops from a focus node" is expressible today.

What is missing is (a) a resident set that is not O(graph) and (b) a backend
that serves radius-`R` adjacency without materialising the graph.

## Decision

Two additive changes, in this order. No exported signature changes; no
semantics change for existing callers.

### Part A — stop storing props twice (resident-set fix)

`Node.Props` / `Edge.Props` remain the live view. In `pkg/storage/bolt`,
`History` becomes load-on-demand:

- Bolt already writes every snapshot to the `history` bucket (`putSnapshot`,
  keyed `elemID\x00<rev>`). Node/edge values stop carrying it: the gob
  written to `nodes`/`edges` has an empty `History` plus `CreatedRev`.
- `GetNodeHistory(id)` / `GetEdgeHistory(id)` serve a prefix scan of the
  `history` bucket — same values, same newest-last order, `nil` on unknown id.
- `GetNodeAt` / `GetEdgeAt` scan the same prefix and stop at the first
  snapshot newer than `t`.
- The `events` bucket already carries the mutation log, so the rev counter no
  longer needs a walk over every snapshot.

Effect: one copy of props per element instead of two, and a history read costs
O(history of that element) instead of O(graph). `inmem` is unchanged — it is
the small-graph default and its behaviour is frozen.

### Part B — focused cache backend (on-demand adjacency)

A new backend `pkg/storage/focused` wrapping any `StorageEngine`:

```
focused.Open(inner StorageEngine, opts Options) *Storage
Options{ Radius int; MaxNodes int; MaxEdges int }
```

- **Resident set** = LRU of nodes and edges, capped by `MaxNodes`/`MaxEdges`,
  seeded from a focus set.
- **Focus** = `Focus(nodeID)` loads the node and its radius-`R` adjacency
  (default `R=2`) by walking `GetEdgesFrom`/`GetEdgesTo` on the inner engine.
- **Miss means expand, never "absent".** A read outside the resident set
  fetches from the inner engine, caches, returns. Only a genuine inner miss
  returns `ok == false`. This is the load-bearing rule: a partial cache must
  never answer "not there" for "not loaded".
- **Enumeration is never cached.** `GetAllNodes`, `GetAllEdges`,
  `GetNodesByLabel`, `GetEdgesByKind`, `GetNodesIn`, `GetEdgesIn` go straight
  to the inner engine, do not populate the cache, and are not served from it.
  They are the export/ingest paths; caching them reintroduces O(graph)
  residency.
- **Writes pass through**, then update or evict the resident copy — never
  leave a stale entry.
- **Versioned reads** (`GetNodeAt`, `GetNodeHistory`, `GetEvents`,
  `EventsTouching`) pass through uncached: rare, and per-element in size.

### What this is not

- Not sharding or a distributed store (`docs/evolution.md` Phase 3).
- Not multi-process: one process, one inner file, as today.
- No DSL change. `Focus` is a backend operation, not a query verb; the query
  engine keeps calling the same `StorageEngine` methods.

## Consequences

- A 52 MB STIX import becomes possible on a 16 GB box: Part A removes one of
  the two prop copies, Part B keeps the working set at the cache bound.
- Exporters must not be handed a `focused.Storage` and expect `GetAllNodes`
  to be cheap — it is a full scan by design, same contract as `bolt`.
- Two new exported names (`focused.Open`, `focused.Options`) are additive to
  the public surface in GOVERNANCE.md; `StorageEngine` is unchanged, so
  axiomflow is unaffected.
- Part A changes bolt's node value (empty inline `History`). The `history`
  bucket already holds the same snapshots, so existing files still read
  correctly; the inline copies are ignored. No format version bump.

## Open decisions

- Default `Radius` (2 vs 3) and the default cache bound — from the PR's
  measurement, not taste.
- Whether `GetNodesByLabel` may seed the cache below a cardinality
  threshold. Default: no.
- Whether `sync` should stream via a `Walk` API instead of `GetAllNodes`.
  Separate concern, out of scope here.

## Verification (filled by the PR)

- `make build && make test` on `sxi-work`.
- Peak RSS for `import --format stix-2.1` of
  `extensions/cti/testdata/enterprise-attack.json` (52 MB) on `inmem`
  (OOM-killed, 14.6 GB), `bolt`, and `focused`.
- A pivot more than `Radius` hops from the focus node, proving
  miss-means-expand.
- Delete-then-read through the cache, proving no stale resident copy.