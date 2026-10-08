# ADR 0003 — BoltDB as the durable storage backend

- **Status:** proposed
- **Date:** 2026-10-08
- **Deciders:** knitknot maintainers

## Context

ADR 0001 established the pluggable backend seam (`Backend` interface +
`--store <scheme>:<path>` selector) and reserved `sqlite:` for a future
durable backend. This ADR selects the first durable backend to implement:
**BoltDB** (`go.etcd.io/bbolt`).

The selection is driven by measured failure of the current `gob + inmem`
backend at enterprise CTI scale, not by preference for a particular
technology:

- **Whole-graph load/save is the cliff.** Every CLI invocation decodes the
  entire `.gob` (233 MB for a 24.5K-object enterprise subset) before any
  query runs — ~4–5 s of the ~4.9 s measured query cost. Import holds the
  parsed bundle + staged maps + 2× props (live + history snapshot) + the
  gob buffer simultaneously, producing multi-GB transient spikes.
- **No durable indexes.** stixID dedup and edge dedup are reconstructed per
  import in application code (the `known`/`seen` maps in
  `extensions/cti/stix_import.go`). They should be schema.
- **Event log triples storage.** Every write stores props three times
  (live + history snapshot + event log), the source of the 10× gob bloat.

BoltDB is chosen over the alternatives because the workload is
small-to-medium graph snapshots with giant occasional values and
append-only history — B+tree territory, not LSM territory:

| Candidate | Verdict | Why |
|---|---|---|
| **BoltDB** | **selected** | Pure Go (no cgo, preserves `go install`); buckets map 1:1 onto the existing inmem index maps; B+tree handles large values without LSM value-log GC; append-only history is natural. |
| SQLite | fallback | Same relational mapping, more conventional, but cgo (mattn) or slower pure-Go (modernc); no advantage over bbolt for this workload. |
| BadgerDB | rejected | LSM + value log is the worst case for multi-KB-to-MB values (37 KB data-components, the 3 MB collection blob); history churn triggers expensive value-log GC; tuning surface contradicts the zero-config promise; archived upstream. |
| DuckDB | rejected as store | Columnar/OLAP is wrong for transactional graph writes; cgo weight. Retained as a possible Parquet *export* sidecar. |

## Decision

Implement a BoltDB backend at `pkg/storage/bolt` that satisfies the existing
`StorageEngine`, `VersionedStorage`, and `metaStore` interfaces, registered
under the `bolt:` URI scheme. The default backend remains `gob + inmem`;
BoltDB is opt-in via `--store bolt:<path>`.

### Bucket layout

bbolt buckets map ~1:1 onto the inmem index maps — this is a port, not a
rewrite. Two buckets make the per-import dedup maps permanent schema.

| Bucket | Key | Value | Replaces (inmem) |
|---|---|---|---|
| `nodes` | nodeID | serialized node | `s.nodes` |
| `edges` | edgeID | serialized edge | `s.edges` |
| `nodesByLabel` | `label\x00nodeID` | — | `s.nodesByLabel` |
| `edgesByFrom` | `from\x00edgeID` | — | `s.edgesByFrom` |
| `edgesByTo` | `to\x00edgeID` | — | `s.edgesByTo` |
| `edgesByKind` | `kind\x00edgeID` | — | `s.edgesByKind` |
| `stixIndex` | stixID | nodeID | **fix 1, durable** |
| `edgeDedup` | `from\x00to\x00kind` | edgeID | **fix 2, durable** |
| `history` | `elemID\x00rev` | snapshot | `node.History` |
| `events` | rev | event | `s.eventLog` |
| `meta` | `revCounter` / `idSeq` | counter | `s.revCounter` / `idSeq` |

`GetNodesByLabel` / `GetEdgesFrom/To/Kind` become bucket prefix scans
(already O(index) in inmem, now durable). `GetAllNodes` / `GetAllEdges`
become full-bucket scans — acceptable, they are rare.

### Phases

**P1 — Core `StorageEngine` (drop-in backend).** Port the 14 methods.
*Exit: `go test ./pkg/storage/...` green against a bolt test variant.*

**P2 — `VersionedStorage`.** `history` + `events` buckets. `GetNodeAt` /
`GetEdgeAt` = prefix scan for latest snapshot ≤ t. *Exit: history cmd +
`TestImportIdempotent` + demo Beat 4 pass on bolt.*

**P3 — `metaStore`.** Source/transaction stamped on each write (the Evolve
path). *Exit: demo Beat 3 Evolve + Beat 4 two-snapshot history pass on bolt.*

**P4 — CLI + migration.** Register `bolt:` in the existing `Selector`
(ADR 0001 seam). Add `gob→bolt` to the sync command. *Exit:
`--store bolt:graph.db` works; `sync gob:demo.gob bolt:out.db` round-trips.*

**P5 — Perf validation.** Re-run the enterprise proxy on bolt: import
should stream (no 250 MB save spike), queries should drop from ~4.9 s to
ms (no 233 MB gob decode). *Exit: numbers recorded.*

### What dissolves

The per-import `known`/`seen` maps in `extensions/cti/stix_import.go`
(fixes 1–2) become unnecessary — `stixIndex` and `edgeDedup` buckets are
queried directly. The import code gets simpler, not just faster. Fix 3
(event-log prop copies) becomes a schema choice (store rev-only, join to
the `history` bucket).

## Risks

- **Serialization format** — gob per node/edge (simple, matches today) vs
  JSON (debuggable, slightly larger). Recommend gob first, revisit if
  debugging hurts.
- **The 3 MB collection blob** — bbolt handles it (B+tree, no LSM value-log
  GC), but it is still one giant value. Backend choice does not fix that
  design question; it just stops it from being fatal.
- **Migration** — gob→bolt sync is the only path; no in-place upgrade.
  Acceptable for a pre-1.0 tool.
- **Single-writer** — bbolt allows one writer at a time; knitknot is
  single-writer CLI/REPL, so no contention. Document it.

## Consequences

- The `gob + inmem` default is unchanged; existing users see no difference.
- Enterprise CTI datasets become viable: bounded-memory import, ms queries,
  durable indexes.
- The `sqlite:` scheme remains reserved; if a relational backend is ever
  needed, the same `StorageEngine` contract applies.
