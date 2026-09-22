# Status

> **Living document.** A point-in-time snapshot of what users can rely on
> with the current build of knitknot. Updated as the codebase changes.
> Last updated: 2026-09-21.

This document is intentionally candid. It describes behavior as it is, not
as we wish it were. If a statement here disagrees with reality, reality wins
and this file is wrong — file an issue or send a PR.

---

## What works

These capabilities are tested by the suite (see [Test coverage](#test-coverage)
below) and have been used against the shipped `data.gob` sample:

- **Fluent DSL**: `Find('Label')`, `Has(rel, value)`, `Where(field, op, value)`,
  `WhereEdge(field, op, value)`, `Limit(n)`, `In(subgraph)`.
- **REPL**: `ADDNODE`, `CONNECT A --rel--> B`, `UPDATE NODE|EDGE`,
  `DELETE NODE|EDGE`, `EXPLAIN`, `DEFINE <verb> TO <Label> VIA <prop>`,
  `LIST VERBS`, `SAVE`, `LOAD`. `exit` and `quit` both save any loaded file
  before closing (and Ctrl+D does too).
- **Persistence**: `Save`/`Load` to `.gob` binary format. The current format
  version is `knitknot/v0.2`; see [Persistence migration](#persistence-migration).
- **Export**: `--format dot`, `--format svg` (requires `graphviz`'s `dot`
  binary), `--format json`. JSON output is a `{nodes, edges, verbs}` document
  with `encoding/json`'s default indentation.
- **Verb registry**: custom relationship semantics via `DEFINE ... TO ...
  VIA ...`; `Has(rel, value)` follows those semantics.
- **Subgraph scoping**: `In('name')` DSL method and `--subgraph name` flag
  both filter query candidates via `GetNodesIn(name)`. A query against a
  subgraph with no nodes of the requested label returns an empty result.
- **Indexing**: `Find('Label')` is O(1) on the label bucket (inmem
  storage maintains a `label -> id -> *Node` map). The query engine's
  non-subgraph candidate fetch uses it.
- **Concurrency model**: `inmem.Storage` is safe for concurrent readers
  (under `RLock`) and a single writer (under `Lock`). The query engine
  reads many times during expansion; see [Concurrency](#concurrency).

---

## What users should not expect

### Scaling

- The label index fixes one O(N) cliff. Edge scans are still linear:
  `GetEdgesFrom`, `GetEdgesTo`, `GetEdgesByKind` iterate the full edge map.
  `Has(...)` expansion is therefore O(edges × matched) per row.
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

- The schema bumped from `knitknot/v0.1` to `knitknot/v0.2` (the `Node`
  and `Edge` `Subgraphs` fields changed from `[]string` to
  `map[string]*Subgraph`).
- `.gob` files saved with older versions will fail to load with an
  `interface conversion` error wrapped with the expected version string.
  There is **no automatic migration**; re-seed via
  `knitknot generate-sample -f data.gob` or write a small migration script.

### Test coverage gaps

| Package                     | Coverage | Gap                                                |
| --------------------------- | -------- | -------------------------------------------------- |
| `pkg/storage/inmem`         | ~91%     | Subgraph mutation paths lightly exercised          |
| `pkg/query`                 | ~80%     | Limit semantics, edge-filter ordering              |
| `pkg/dsl`                   | ~76%     | Fuzz target exists; not in CI                      |
| `pkg/graph`                 | ~73%     | `WithQueryEngine`, `WithVerbs` paths untested      |
| `cmd`                       | ~23%     | `exportToSVG`, `printExplain` happy paths          |
| `pkg/exporter/dot`          | **0%**   | The visualization output path has no tests         |

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

---

## Domain extensions

Business-domain behavior has its own living status docs:

- [Threat Intelligence](docs/domains/threat-intelligence/STATUS.md) —
  STIX 2.1 import is implemented (Stage 1); no version history, no STIX
  export, markings stored but not enforced.

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

When you change a capability, add a feature, or close a limitation:
update the relevant section here in the same PR. Don't let it drift.
If you change a foot-gun's behavior, **delete** the foot-gun entry and
move it to the changelog; do not leave a stale warning.
