# ADR 0002 — Append-only versioning across all elements

- **Status:** proposed
- **Date:** 2026-09-22
- **Deciders:** knitknot maintainers

## Context

KnitKnot's graph is currently a snapshot. `UpdateNode` and `UpdateEdge`
replace the props map; `DeleteNode` and `DeleteEdge` remove the entry.
This works for the analyst / REPL / single-process niche the README and
STATUS.md describe, but it loses information that several plausible use
cases need:

- **CTI**: a STIX `Indicator` may be re-issued; confidence changes;
  `revoked` becomes true; a new relationship reframes the same object.
  The CTI blueprint explicitly calls versioning the "most serious CTI
  issue" because re-importing an updated SDO currently destroys the old
  claim.
- **Identity**: a person's role, email, team membership, and
  authentication status change over time.
- **Healthcare**: medication history, diagnoses, treatment outcomes.
- **Fraud**: risk-score evolution, KYC status changes.
- **Configuration / infrastructure**: what a service looked like at
  deploy N; which node ran which version of which container.
- **Knowledge graphs**: claim corrections, source deprecations.

If versioning is implemented inside the CTI extension, every future
domain reinvents it badly and the answer to "what did this thing look
like last month" depends on which extension the user happened to be in.
If versioning is implemented in the graph kernel, every domain gets it
for free and the kernel remains the natural place to ask temporal
questions about any element.

This ADR is the kernel change. The CTI extension rides it without
modification. Other future domains benefit without re-deciding.

## Two indexes, not one

Discussions surfaced two natural axes for versioning: **time** and
**event-point (revision)**. This ADR supports both because they answer
different questions:

- Time answers "what did the graph look like at wall-clock T?"
  Requires timestamps; sensitive to clock skew and backdated imports.
- Revision answers "what did the graph look like at event N?"
  Monotonic, stable under clock issues, useful for diffs.

For domains like CTI, two timestamps per event are needed because they
are not the same:

- **LogicalTime** — when the change was true in the source's world
  (e.g. STIX `modified: 2024-03-22`).
- **EventTime** — when the graph recorded it (e.g. import run at
  2024-06-01 14:32 UTC).

The revision counter is just an `int64` that increments per mutation
and is monotonic per `Storage` instance. Mapping "rev N happened at
wall-clock T" is a function of the event log, not a separate concept.

## Mark-delete, never real deletion

The "delete" operation is a tombstone, not a removal. A node or edge
deleted at time T continues to exist in storage; its `DeletedAt` field
records when. This makes the questions you can ask about it clean:

- "What is node3's state immediately after node17 was deleted?"
  `GetNodeAt(node3.ID, node17DeleteEvent.EventTime)`.
- "What events happened in the same operation as node17's deletion?"
  `GetEventsByTransaction(node17DeleteEvent.Transaction)`.
- "Did node3 exist before node17's deletion?" Compare node3's history
  against the delete event's time.

Real deletion would lose history, which is the entire point of this
ADR. Tombstones are a small constant cost per element that was ever
deleted; memory is bounded by element count, not version count.

## Decision

Introduce append-only versioning as a property of every `Node` and
`Edge`, with a single per-`Storage` event log indexing both by
revision and by element.

### Data model

```go
// pkg/ports/types/graph.go (additions)

type Snapshot struct {
    Rev         int64                  // monotonic per Storage instance
    EventTime   time.Time              // when we recorded it
    LogicalTime time.Time              // when it was true in the source's world
    Props       map[string]any         // full property snapshot at this Rev
    Source      string                 // who/what caused the event (feed id, "user", etc.)
    Deleted     bool                   // tombstone marker (only meaningful when true)
    Transaction string                 // groups events from the same operation
}

type Event struct {
    Rev         int64
    EventTime   time.Time
    LogicalTime time.Time
    Op          string                 // "create" | "update" | "delete"
    ElementType string                 // "node" | "edge"
    ElementID   string
    From        string                 // for edge events
    To          string                 // for edge events
    Kind        string                 // for edge events
    Props       map[string]any         // snapshot at this Rev
    Source      string
    Transaction string
}

type Node struct {
    // ... existing fields ...
    History    []Snapshot             // append-only; len ≥ 1
    DeletedAt  *time.Time             // nil = live
    CreatedRev int64                  // first Revision
}

type Edge struct {
    History    []Snapshot
    DeletedAt  *time.Time
    CreatedRev int64
}
```

`History` always has at least one entry — the current state. `DeletedAt`
is `nil` for live elements, non-nil for tombstones. `CreatedRev` makes
it cheap to ask "when was this thing born."

### Storage contract

The existing `StorageEngine` interface is unchanged. A new optional
interface lives alongside it:

```go
// pkg/ports/storage/versioned.go (new file)

type VersionedStorage interface {
    StorageEngine

    // Element at a point in time
    GetNodeAt(id string, t time.Time) (*types.Node, bool)
    GetEdgeAt(id string, t time.Time) (*types.Edge, bool)

    // Full history (newest-last; first entry is CreatedRev)
    GetNodeHistory(id string) []types.Snapshot
    GetEdgeHistory(id string) []types.Snapshot

    // Event log queries
    GetEvents(fromRev, toRev int64) []types.Event
    GetEventsByTransaction(txID string) []types.Event
    EventsTouching(elementID string) []types.Event
}
```

Backends that support versioning implement this; backends that don't
simply don't (callers fall back to `StorageEngine`). `inmem` implements
it in this ADR; SQLite will when it ships.

### Mutation semantics

- `UpdateNode(id, props)` becomes: append a new `Snapshot` to
  `node.History` with the merged props; bump `Rev`; append an `Event`
  with `Op="update"`. The "current" view of the node is the last entry
  in `History`. Existing callers that just read `node.Props` see the
  latest snapshot unchanged — no caller code breaks.
- `DeleteNode(id)` becomes: append a `Snapshot{Deleted: true, Props:
  node.Props}` to `node.History`; set `node.DeletedAt = &eventTime`;
  append an `Event{Op="delete"}`. The element is still findable via
  `GetNode(id)` (which returns it with `DeletedAt != nil`); callers
  filter as they wish. `GetNode(id).Label` etc. still work.

  Rationale: hiding tombstones from `GetNode` would break the
  `findEdges` / `GetEdgesFrom` paths that already iterate stored
  elements. A caller that wants "live only" filters on `DeletedAt`.
- `AddNode(label, props)` creates the node with `History = [{Rev, ...,
  Props: props}]`, `CreatedRev = Rev`. Same shape as before, just
  history-aware.
- `AddEdge(...)` mirrors `AddNode`.
- Every mutation increments a per-`Storage` revision counter and
  appends an `Event`. The event log is mutex-protected (existing
  inmem-style locking; SQLite will use WAL transactions).

### Read semantics

- `GetNode(id)` returns the element with the latest live snapshot
  (or the tombstone snapshot if the element is deleted). Existing
  callers see no change.
- `GetNodeAt(id, t)` returns the element as of time `t`. Semantics:
  - Find the latest snapshot whose `EventTime <= t`.
  - If that snapshot is a tombstone, return not-found (the element
    didn't exist at `t`).
  - Otherwise return the element with that snapshot's props.
- `GetNodeAt(id, time.Time{})` (zero value) is invalid and returns
  not-found. Callers should use `GetNode` for the current view.
- `GetNodeHistory(id)` returns the element's full history, newest-last.
- `GetEvents(fromRev, toRev)` returns the event list in `[fromRev,
  toRev]` (inclusive), ordered by `Rev` ascending. `toRev == 0` means
  "to current rev".
- `GetEventsByTransaction(txID)` returns events sharing that
  transaction ID, in `Rev` order. Used for causal-neighborhood
  queries ("what else happened in the same op").
- `EventsTouching(elementID)` returns events whose `ElementID ==
  elementID`, in `Rev` order. Used to reconstruct the full lifecycle
  of one element.

### Persistence

The gob format bumps from `knitknot/v0.2` to `knitknot/v0.3`. The
`data.gob` file regenerates.

- `Node` and `Edge` gain `History []Snapshot` and `DeletedAt *time.Time`
  and `CreatedRev int64`; gob's default struct-tag rules encode these
  cleanly.
- The event log persists as a top-level field on `SavedGraph`.
- Legacy `.gob` files (v0.2 and earlier) load as follows: every node
  and edge gets one synthetic snapshot with `Rev=1`,
  `LogicalTime=time.Time{}` (epoch / "we don't know"), `Source="legacy"`,
  `Deleted=false`. Legacy data has no event log; queries against
  history on legacy data return only this one synthetic snapshot per
  element. This is the documented, honest answer.

### What stays the same

- The `StorageEngine` interface (`AddNode`, `GetNode`, `GetAllNodes`,
  `UpdateNode`, `DeleteNode`, etc.) is unchanged. Existing callers
  compile and behave as before.
- The graph query DSL is unchanged. `Find('malware')` still returns
  all live malware; new time-aware queries are added separately if
  at all (out of scope for this ADR).
- The existing REPL and CLI commands work unchanged.
- Domain extensions (CTI, future) need no modification. The CTI
  Stage 1 importer's stable-ID upsert continues to work; what changes
  is that `UpdateNode` no longer destroys history.
- The exporter's default (latest-version) behavior is unchanged.

### What this ADR does not decide

These are explicit deferrals. Each is its own future ADR if/when
needed:

- **A temporal query DSL** (`Find('malware').AsOf('2024-04-01')`).
  Build on top of the primitives; don't bake into the kernel.
- **Edge versioning in v1.** The shape mirrors nodes, but this ADR's
  implementation lane scopes the first cut to nodes; edges follow.
- **Explicit transaction API** (`WithTransaction(func(){ ... })`).
  v1 auto-groups by per-call transaction ID; an explicit API wraps
  without breaking.
- **Subgraph versioning.** Subgraphs already have a separate
  lifecycle; versioning them is a third axis.
- **Cross-process event log.** Single-process for now. Multi-process
  is the SQLite/rack-server step.
- **Time-range queries** (`AsOfRange`, "show me all nodes that
  existed at any time in [T1, T2]"). Useful but not required for the
  immediate CTI use case.
- **Per-property diff views** ("show me which properties changed
  between rev.X and rev.Y on this node"). Derivable; not built.
- **Retention / GC policy** for the event log. v1 keeps everything.
  Memory grows monotonically with mutations; document the cost.
- **Clock-source policy** (UTC always? RFC3339Nano?). v1 stores
  `time.Time` as-is; importers normalize. Document the recommendation.
- **Soft-delete vs hard-delete opt-in.** v1 is tombstones-only.
- **Reverse event log queries** (newest-first, pagination). v1 is
  ascending only.
- **Compression / serialization optimization** for large `History`
  slices. Gob handles them; large-scale optimization is a future
  concern.

## Consequences

### Positive

- CTI versioning becomes free. Re-importing an updated STIX object
  keeps v1 and v2 instead of overwriting. The blueprint's "destructive
  prop merge" footnote becomes obsolete.
- Every future domain gets versioning without re-deciding. Identity,
  healthcare, fraud, configuration graphs all benefit.
- The audit log is intrinsic. Forensics questions like "what was
  the state before this incident?" become first-class queries.
- Point-in-time export becomes possible (`GetNodeAt(id, T)` + emit).
  Useful for compliance.
- The shape is testable. Every primitive has a clear contract; the
  test surface is mechanical.

### Negative

- Storage grows by one event per mutation plus one snapshot per
  element change. The cost is honest: a CTI bundle with 1000 objects
  produces at least 1000 events.
- The kernel's public surface grows by one optional interface
  (`VersionedStorage`) and one type (`Snapshot`, `Event`). Maintenance
  surface is small but real.
- Mutation paths that mutate `node.Props` directly (bypassing
  `UpdateNode`) become even more wrong than they are today. The
  audit's M4 concern (leaky pointers) becomes a stronger guarantee.
- Future work (temporal query DSL, retention policy) is implied by
  this ADR but not delivered.

### Mitigation

- Document the memory cost in the kernel's package doc.
- The `VersionedStorage` interface is optional; backends opt in.
- The first implementation lane is tightly scoped: types + inmem +
  migration + tests + a note in `extensions/cti/STATUS.md`. No DSL,
  no retention policy, no SQLite work.
- The CTI consequences are explicitly negative — "Stage 1 importer's
  destructive merge" is no longer accurate. Update that doc in the
  same PR.

## Decision outcome

Adopt. Land as one PR containing:

1. `pkg/ports/types/graph.go` — add `Snapshot`, `Event` types; add
   `History`, `DeletedAt`, `CreatedRev` fields to `Node` and `Edge`.
2. `pkg/ports/storage/versioned.go` — `VersionedStorage` interface.
3. `pkg/storage/inmem/inmem.go` — implement versioning: per-instance
   event log with mutex; `UpdateNode`/`DeleteNode` append; new methods
   on `VersionedStorage`. Locking unchanged (existing mutex pattern).
4. `pkg/storage/inmem/persist.go` — gob format bump to `v0.3`; save
   and restore the event log; legacy migration synthesis.
5. `pkg/storage/inmem/*_test.go` — append-on-update, GetNodeAt
   past/future/missing, tombstone semantics, event log ordering,
   transaction grouping, legacy migration.
6. `data.gob` regenerated.
7. `extensions/cti/STATUS.md` — "destructive merge" updated to
   "non-destructive; history preserved."

Out of scope for this PR: edge versioning, temporal DSL, retention
policy, SQLite, explicit transaction API. Each is a separate ADR if
needed.

## Open decisions worth resolving before implementation

These are smaller than the ADR's main decisions but user-visible:

1. **Legacy migration timestamp.** `time.Time{}` (epoch / "unknown")
   or `LoadedAt` (now / "no real history")? **Recommendation: epoch.**
   Legacy data clearly has no real history; "unknown" is honest.
   LoadedAt would be misleading.

2. **History visibility by default.** Does `GetNode` return the node
   with `History` attached (so a caller can read it without a second
   query) or a slim view (forcing callers to opt in)?
   **Recommendation: attached.** The slice header is nil for nodes
   with one version, so the cost is near-zero; the convenience is real.

3. **Transaction ID source.** Auto-generated UUID per call, or
   monotonic int64? **Recommendation: UUID.** Easier to correlate
   across processes in future debug logs; cost is negligible.

4. **Clock policy.** Insist on UTC? Trust the caller? **Recommendation:
   store as `time.Time` (UTC if the caller passes UTC; non-UTC is
   preserved as-given); document the recommendation. Importer code
   normalizes to UTC before calling the kernel.**

5. **`Rev` zero-value semantics.** Is `Rev=0` "no revisions" (invalid)
   or "first revision"? **Recommendation: revisions start at 1.**
   `Rev=0` is a sentinel for "no history."