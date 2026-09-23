# Status

> **Living document.** A point-in-time snapshot of what users can rely on
> with the current build of knitknot. Updated as the codebase changes.
> Last updated: 2026-09-23.

This document is intentionally candid. It describes behavior as it is, not
as we wish it were. If a statement here disagrees with reality, reality wins
and this file is wrong — file an issue or send a PR.

---

## What works

> [updated 2026-09-23] Added: extensions framework, CTI Stage 1+2,
> `--store`, versioning, `--source`, DOT hardening.

These capabilities are tested by the suite (see [Test coverage](#test-coverage)
below) and have been used against the shipped `data.gob` sample:

- **Fluent DSL**: `Find('Label')`, `Has(rel, value)`, `Where(field, op, value)`,
  `WhereEdge(field, op, value)`, `Limit(n)`, `In(subgraph)`.
- **REPL**: `ADDNODE`, `CONNECT A --rel--> B`, `UPDATE NODE|EDGE`,
  `DELETE NODE|EDGE`, `EXPLAIN`, `DEFINE <verb> TO <Label> VIA <prop>`,
  `LIST VERBS`, `SAVE`, `LOAD`. `exit` and `quit` both save any loaded file
  before closing (and Ctrl+D does too).
- **Persistence**: `Save`/`Load` to `.gob` binary format. The current format
  version is `knitknot/v0.4` (v0.3 added node `History`/`DeletedAt`/
  `CreatedRev` plus the top-level event log; v0.4 extended the same shape
  to edges); see [Persistence migration](#persistence-migration).
- **Export**: `--format dot`, `--format svg` (requires `graphviz`'s `dot`
  binary), `--format json`. JSON output is a `{nodes, edges, verbs}` document
  with `encoding/json`'s default indentation.
- **DOT exporter hardening**: node IDs are DOT-quoted identifiers (spaces,
  dots, quotes, leading digits, non-ASCII safe); labels use DOT-style
  escaping (backslash, quote, newline); node and edge output is sorted by ID
  for stable diffs; duplicate emissions dedup on mapped node names plus the
  storage-assigned edge ID.
- **Verb registry**: custom relationship semantics via `DEFINE ... TO ...
  VIA ...`; `Has(rel, value)` follows those semantics.
- **Subgraph scoping**: `In('name')` DSL method and `--subgraph name` flag
  both filter query candidates via `GetNodesIn(name)`. A query against a
  subgraph with no nodes of the requested label returns an empty result.
- **Indexing**: `Find('Label')` is O(1) on the label bucket (inmem
  storage maintains a `label -> id -> *Node` map). The query engine's
  non-subgraph candidate fetch uses it.
- **Extensions framework** (`extensions/extension.go`): compiled-in
  `Extension` / `Importer` / `Exporter` / `Registry` contract with
  `NewRegistry()`. Duplicate, empty, and nil registrations are rejected;
  post-`Freeze()` writes are rejected. `knitknot import --format <name>`
  dispatches through the registry; the CTI extension wires in via the
  `registerExtensions` hook.
- **CTI extension Stage 1+2**: STIX 2.1 bundle import (SDOs, SCOs as generic
  nodes, relationships, sightings, markings, language content, `x-*` custom
  objects; order-independent two-pass; idempotent re-import; fail-before-
  mutate) plus STIX 2.1 bundle export (graph → bundle; only nodes/edges
  carrying a valid STIX ID export, the rest is skipped and counted, never
  fabricated). Details live in
  [Threat Intelligence](docs/domains/threat-intelligence/STATUS.md).
- **Pluggable storage backends** (ADR 0001): `--store <scheme>:<path>`
  selects the backend (`gob:` shipped via inmem; `sqlite:`/`mem:` reserved
  for the future). `-f, --file` is preserved as a synonym for
  `--store gob:<path>`; `--store` wins when both are given. Wired into
  `import`, `export`, `query`, and `repl`. `knitknot sync --from <uri>
  --to <uri>` exists as a reserved stub and returns "sync not implemented"
  until a backend pair ships a merge.
- **Append-only versioning** (ADR 0002): every node and edge carries
  `History []Snapshot` (len ≥ 1), `DeletedAt`, and `CreatedRev`; every
  mutation appends a snapshot and an entry on a single per-`Storage` event
  log (`Rev`, `EventTime`, `LogicalTime`, `Op`, `Source`, `Transaction`).
  Current-state reads (`GetNode`, `Props`) see the latest snapshot
  unchanged; temporal reads (`GetNodeAt`/`GetEdgeAt`, `GetNodeHistory`/
  `GetEdgeHistory`, `GetEvents`, `GetEventsByTransaction`,
  `EventsTouching`) expose the past. Re-importing an updated object appends
  a snapshot instead of destroying the old claim.
- **Tombstone deletes**: `DeleteNode`/`DeleteEdge` mark rather than remove.
  Deleted elements stay findable via `GetNode`/`GetEdge` with
  `DeletedAt != nil`; live-only enumeration (`GetAllNodes`,
  `GetNodesByLabel`, `GetAllEdges`, edge scans, subgraph reads) skips them.
  Deleting a node tombstones its incident edges in the same transaction.
  Updating or re-deleting a tombstone errors.
- **Feed provenance**: `knitknot import --source <feed-id>` records
  `source_feed` on every imported node and edge and flows it into the
  versioning `Snapshot.Source` / event-log `Source`.
- **Concurrency model**: `inmem.Storage` is safe for concurrent readers
  (under `RLock`) and a single writer (under `Lock`). The query engine
  reads many times during expansion; see [Concurrency](#concurrency).

---

## What users should not expect

### Scaling

> [updated 2026-09-23] Label index confirmed; edge scans still linear.

- The label index fixes one O(N) cliff, label lookups only: `Find('Label')`
  and the non-subgraph candidate fetch are O(1) on the label bucket. Edge
  scans are still linear: `GetEdgesFrom`, `GetEdgesTo`, `GetEdgesByKind`
  iterate the full edge map. `Has(...)` expansion is therefore O(edges ×
  matched) per row.
- No benchmark suite ships. The 100K-nodes goal in `docs/ROADMAP.md` is
  unverified. At ~10K nodes queries feel responsive; the next wall is not
  pinned to a number.
- All filtering happens after expansion in the non-subgraph path. A query
  like `Find('customer').Has('purchased', 'X').Where('n.age', '>', 30)`
  expands every customer's purchases before the age filter runs.

### Concurrency

- Safe pattern: many readers + one writer goroutine per `Storage`.
- Not safe: multi-writer (no row-level locking, no transactions) or
  long-running concurrent reads during a writer (an edge can be deleted
  between `GetEdgesFrom` and the subsequent `GetNode(e.To)`, leaving
  the query to skip it — no panic, but inconsistent results are possible).
- The concurrency test (`pkg/storage/inmem/concurrency_test.go`) hammers
  the same hot path; it does not exercise realistic mixed-reader/writer
  workloads.

### Persistence migration

> [updated 2026-09-23] v0.3 (node versioning) and v0.4 (edge versioning)
> are now the migration story; v0.4 is current.

- The schema bumped from `knitknot/v0.1` to `knitknot/v0.2` (the `Node`
  and `Edge` `Subgraphs` fields changed from `[]string` to
  `map[string]*Subgraph`), then to `knitknot/v0.3` (nodes gained
  `History`/`DeletedAt`/`CreatedRev`, plus the top-level event log and rev
  counter), then to `knitknot/v0.4` (edges gained the same shape).
  `knitknot/v0.4` is current.
- `.gob` files saved with older versions will fail to load with an
  `interface conversion` error wrapped with the expected version string.
  There is **no automatic migration**; re-seed via
  `knitknot generate-sample -f data.gob` or write a small migration script.
  (Legacy files pre-versioning conceptually load with one synthetic
  `Rev=1`, `Source="legacy"` snapshot per element — see ADR 0002 — but no
  loader implements that synthesis today; old files simply fail to load.)

### Test coverage gaps

> [updated 2026-09-23] DOT exporter now covered (was 0%); new rows for
> `extensions`, `extensions/cti`, `pkg/ports/storage`.

| Package                     | Coverage | Gap                                                |
| --------------------------- | -------- | -------------------------------------------------- |
| `pkg/storage/inmem`         | ~91%     | Subgraph mutation paths lightly exercised          |
| `pkg/query`                 | ~80%     | Limit semantics, edge-filter ordering              |
| `pkg/dsl`                   | ~76%     | Fuzz target exists; not in CI                      |
| `pkg/graph`                 | ~73%     | `WithQueryEngine`, `WithVerbs` paths untested      |
| `cmd`                       | ~23%     | `exportToSVG`, `printExplain` happy paths          |
| `pkg/exporter/dot`          | covered  | Quoting/escaping, sorted output, and dedup specs ship (previously 0%) |
| `extensions`                | covered  | Registry contract: duplicate/empty/nil/frozen rejects |
| `extensions/cti`            | covered  | STIX import round-trip + export specs against the `minimal-bundle.json` fixture |
| `pkg/ports/storage`         | covered  | Contract check that inmem implements `VersionedStorage` |

Coverage figures are from `go test -cover ./...` and are statement counts,
not branch coverage. Numbers drift with every change.

### Foot-guns the docs don't flag

- **`.Has(...)` only follows outgoing edges.** There is no inverse
  direction. If you ask "what purchases did 'Marketplace' have" expecting
  the inverse of `customer → channel`, you get nothing. Either think
  about which direction the data flows, or pre-register verbs for the
  reverse direction and `Has` those.
- **`WhereEdge` only attaches to the *last* `Has` edge.** A chain
  `Has('a', ...).WhereEdge('foo', '=', 1).Has('b', ...)` filters edge
  `b`, not edge `a`. There is no way to filter intermediate edges in a
  multi-step traversal.
- **Empty subgraphs return no results.** `--subgraph foo` against a
  fresh install returns the empty result — correct, but indistinguishable
  from "the graph has no matches" to a user who hasn't added anything to
  `foo`.
- **Numeric vs string properties matter at write time.** Seed data
  stores `age` as the string `"51"`. With the M1 fix, `Where('n.age',
  '=', 51)` now matches the string-stored value, but only because
  `compare` coerces. If you write a numeric 51 elsewhere, the comparison
  still works because coercion is symmetric — but a JSON round-trip
  re-marshals the value and your downstream pipeline may not. The DSL
  treats both the same; external tools might not.

### Dead API surface (kept for stability)

The following are un-called from the rest of the codebase. They are
**not** deprecated — removing them would be a breaking change for
embedders — but they have no current consumer:

- `StorageEngine.GetEdgesTo`, `GetEdgesByKind`
- `QueryPlan.Outputs`, `QueryPlan.OffsetVal` (set in some places, never read)
- `GraphEngine.WithQueryEngine`, `GraphEngine.WithVerbs`,
  `Builder.In` (used by the H3 fix — actually *live*, ignore this line;
  `WithQueryEngine` and `WithVerbs` are still dead)

If you are embedding the library and can confirm none of your code uses
them, file an issue and we can deprecate them in a release.

---

## For embedders

> [updated 2026-09-23] Extension framework is the way to add a domain;
> `VersionedStorage` is opt-in.

- The public API in `pkg/ports/storage/storage.go` and `pkg/graph/engine.go`
  is small and stable. The query engine interface is also stable.
- Treat `*inmem.Storage` as **per-process**. It is not a shared database,
  has no networking layer, and has no durability beyond `.gob` save/load.
- Concurrency: the only tested safe pattern is one writer + many readers.
  Treat it as a per-account mutex or a single-writer goroutine.
- The DSL grammar is documented in `docs/dsl.md`. Anything outside the
  documented methods (`Find`, `Has`, `Where`, `WhereEdge`, `Limit`, `In`)
  requires going around the DSL and using `Builder` directly.
- Public type definitions live in `pkg/ports/types` (`Node`, `Edge`,
  `Subgraph`, `Verb`). Don't redeclare them in your code.
- To add a domain, implement the extension framework
  (`extensions/extension.go`: `Extension` + `Importer`/`Exporter` +
  `Registry`), not a fork of `cmd` or storage. See
  [docs/domains/README.md](docs/domains/README.md) for the boundary rules
  (kernel stays domain-neutral; extensions depend on graph ports, never on
  `cmd` or a concrete storage package).
- Versioning is opt-in at the storage layer: `VersionedStorage` in
  `pkg/ports/storage/versioned.go` extends `StorageEngine` with temporal
  reads and the event log. `inmem` implements it; backends without it still
  satisfy `StorageEngine` and everything else keeps working. The graph
  engine passes the `StorageEngine` through unchanged.

---

## Domain extensions

> [updated 2026-09-23] STIX export shipped; version history landed; markings
> note moved to the CTI doc.

Business-domain behavior has its own living status docs:

- [Threat Intelligence](docs/domains/threat-intelligence/STATUS.md) —
  STIX 2.1 import (Stage 1) and STIX 2.1 export (Stage 2) are implemented;
  re-imports preserve append-only version history (ADR 0002) instead of
  destroying the old claim. Markings are stored but not enforced — that
  remains the CTI doc's concern, not this file's; see its "Markings
  stored, not enforced" section before showing anything to a compliance
  officer.

See [docs/domains/README.md](docs/domains/README.md) for the boundary
rules new domains must follow.

---

## Documentation gaps

These still ship with `TODO` placeholders:

- `docs/ARCHITECTURE.md` has `TODO: *draw component interaction diagram*`
  and `TODO: *need edit*`.
- The DSL railroad diagram in `docs/dsl-syntax.md` is not yet drawn;
  the file points at the BNF section instead.

The README used to claim 86% coverage; the badge was removed because
the figure was unreliable. Regenerate via `./cov.sh` if you need a
current number.

---

## How to update this file

> [updated 2026-09-23] This file is now living and substantial: tag changed
> sections, keep the candid tone, don't restructure.

When you change a capability, add a feature, or close a limitation:
update the relevant section here in the same PR. Don't let it drift.
If you change a foot-gun's behavior, **delete** the foot-gun entry and
move it to the changelog; do not leave a stale warning.
Mark each changed section with a brief `[updated YYYY-MM-DD]` tag at its
top so the next editor can see what moved and when. Keep the existing
sections, order, and candid tone — refresh content in place rather than
restructuring; new capabilities belong as bullets under "What works" with
their test packages named in "Test coverage", and closed limitations get
edited where they stand, not appended elsewhere.
