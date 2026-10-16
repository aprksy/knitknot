# Memory (AI Episodic Memory) — knitknot as Storage Substrate

> **Status:** proposed
>
> **Owner:** knitknot maintainers (substrate) + AxiomFlow team (injected slots)
>
> **Last reviewed:** 2026-10-15

This document is a **contract-spec proposal**: it defines the boundary
between knitknot (graph storage, lineage, versioning, retrieval primitives)
and AxiomFlow (embeddings, LLM reasoning, vector indexing, syscall wiring)
for implementing the Semantic Forest memory subsystem
(`docs/semantic-forest.md`, foreign spec, untracked) on knitknot.

Rule of the contract: **knitknot never calls an LLM, never computes an
embedding, never runs similarity math.** It stores nodes, edges, versions,
and answers structural queries. Everything semantic is injected.

## Problem statement

An AI runtime needs durable, per-user episodic memory: conversation turns,
summaries, and topic clusters, organized as forests that can be retrieved
by recency, topic, lineage, and semantic similarity. The workload is
small-to-medium (their spec: brute force acceptable under ~10k nodes/user),
single-writer per user, and read-heavy — squarely inside knitknot's
analyst-sized design envelope.

A property graph is suitable because the core operations are structural:
parent/child lineage, depth-bounded subtree retrieval, and version chains.
Similarity ranking is explicitly *not* a graph operation and lives outside.

## Scope

### In scope (knitknot provides)

- `ContextNode` storage as labeled nodes; `parent`/`child` lineage as edges
- Per-user scoping (subgraph per user)
- Append-only version history per node/edge (immutable payloads stay immutable)
- Lineage queries (node→root path), depth-bounded subtree retrieval
- Type/tagged filtering expressible in the DSL
- Time-range filtering over epoch numbers and RFC3339 strings
- Crash-safe single-file persistence (bbolt backend)
- Versioned export (Parquet) for the out-of-band indexer

### Out of scope (AxiomFlow fills, or stays out)

- Embedding computation, LLM summarization/topic extraction
- Vector similarity search and ANN indexing
- Full-text search (`contains`/regex do not exist in the DSL)
- Array-membership filtering (e.g. `tags CONTAINS x`) — not expressible;
  materialize tag edges or filter in the adapter layer
- Multi-writer concurrency, access control, TLP-style enforcement

## Domain model

### Node types

| Type (label) | Identity | Required properties | Notes |
| --- | --- | --- | --- |
| `user_profile` | stable `memory_id` | `user_id` | one root per user (convention, not enforced) |
| `session_summary` | stable `memory_id` | `user_id`, `payload` | L1/L2 summaries |
| `pr_turn` | stable `memory_id` | `user_id`, `payload` | raw prompt/response pair |
| `topic_cluster` | stable `memory_id` | `user_id`, `payload` | refactoring output |
| `tag` | tag string (`name`) | `name` | one node per distinct tag value; target of `tagged` edges |

Node IDs are **storage-assigned** (sequential); the stable cross-system key
is a `memory_id` prop, looked up with `Where('n.memory_id', '=', ...)`.
This mirrors the established `stix_id` pattern.

### Edge types

| Kind | From | To | Required properties | Direction |
| --- | --- | --- | --- | --- |
| `child-of` | child node | parent node | none | Outgoing (child → parent) |
| `similar-to` | node | node | `score`, `model`, `observed_at` | Outgoing; written back by the indexer, never by knitknot |
| `supersedes` | new version | old version | none | Optional; alternative to prop-only supersession |
| `tagged` | node | tag | none | Outgoing (node → tag); the DSL-queryable form of `tags` |

Lineage in either representation; **edges, not `parent_id` props** — pick one
and do not store both (the graph is the source of truth for structure).

### Property conventions

- `payload`: structured map (never raw text). Content bytes live in the
  backend blob area, content-addressed; the node holds `payload_ref`
  (`sha256:<hex>`). v1 nodes may still carry small payloads inline (see
  format versions below).
- `embedding`: small embeddings (serialized ≤ 4 KiB, tunable — e.g. 384-dim
  float32 ≈ 1.5 KiB) live inline as an opaque `[]float32` prop knitknot never
  reads; large embeddings live in the backend blob area and the node holds
  `embedding_ref` + `embedding_model` + `embedding_dim` instead (see below).
- `embedding_model`, `embedding_dim`: recorded on the node at write time so
  a model change is detectable instead of silently mixing vector spaces.
- `format`: `"v1"` (default) or `"v2"` — explicit version marker; see below.

## Payload split and format versions

Large, opaque bytes (conversation text, raw payloads, embedding vectors) do
not live in the graph. knitknot owns structure, identity, traversal, and
versions; bulk bytes live in the backend blob area, content-addressed, and
the node holds a reference. This is both a performance decision — graph
load/save and history snapshots scale with stored bytes, and measured data
shows props dominate at scale — and an ownership decision: text, embeddings,
and full-text search belong to the LLM/embedding/FTS domain, not the graph.

Content addressing: `sha256:<hex>` (or equivalent), never bare paths — dedup
is free, verification is trivial. The blob area is backend-scoped: on bolt it
is a `blobs` bucket keyed by hash (**proposed** — same file, same
transaction, so no two-store skew); on gob blobs ride in the file; an
external store (file, SQLite, S3) is interchangeable wherever the backend
supports it.

Size rules (defaults, tunable per store):
- **vectors**: inline as an opaque prop while serialized ≤ 4 KiB, else
  `embedding_ref` + `embedding_model` + `embedding_dim`.
- **content**: always referenced (`payload_ref`); small payloads may stay
  inline on v1 (see below).

| Version | Payload | Embedding | Default? |
| --- | --- | --- | --- |
| v1 | inline props | inline opaque prop, or absent | **yes** |
| v2 | `payload_ref` (content hash) | size rule above | opt-in per write |

Detection is structural (`payload_ref` present ⇒ v2) plus the explicit
`format` prop for forward clarity. Mixed v1/v2 graphs are legal by design.

References are **content-addressed** (`sha256:<hex>` or equivalent), never
bare paths — dedup is free, verification is trivial, and the backing store
(file, SQLite, S3) is interchangeable.

Migration is lossless in both directions: v1→v2 moves bytes out and writes
refs under a new snapshot (so rollback is a revert, not a reconstruction);
v2→v1 inlines refs back for single-file portability.

Risks carried explicitly: join-back cost on full exports (traversal keeps
the whole gain; exports pay per need); two-store crash skew (mitigated by
content hashes + versioned refs); vector-space mixing (`model`+`dim`
recorded at write; mismatch errors instead of degrading silently).
- `created_at`: **epoch number or RFC3339 string** — both filter
  chronologically in `Where` (`>`, `<`, `=`, `!=`).
- `tags`: stored as-is but **not filterable** (no array ops); for queryable
  tags, materialize `tagged` edges to tag nodes, or filter in Go.
- `version`, `is_stale`, `superseded_by`: plain props; supersession is a
  new node (or new snapshot) plus a `supersedes` edge / stale flag, never a
  payload rewrite.

## Identity and deduplication

- Stable identity = `memory_id` (caller-assigned, e.g. trace/turn UUID).
- Upsert by `memory_id` lookup; updates **append a snapshot**, never mutate
  history (ADR 0002 semantics).
- Embedding re-computation under a changed `embedding_model`/`dim` must
  either rewrite or version the node — never silently mix spaces.

## Temporal model

- `created_at` (epoch/RFC3339) supports range filtering today.
- Point-in-time views use the kernel primitives (`GetNodeAt`,
  `GetNodeHistory`) — there is no `.AsOf(...)` DSL surface.
- No event sourcing beyond snapshots; no retention policy (same as core).

## Provenance

- `source_trace_id` (their Petri-Net link): plain prop, carried on every
  node/edge; no validation.
- `source_feed`-style feed attribution from the CTI/DFIR convention applies
  if memory is imported in batches.

## Validation

- Structural invariants (valid edge endpoints, unique storage IDs) are the
  kernel's; domain rules (payload shape, allowed parent/child type pairs,
  `memory_id` presence) belong in the adapter, never in the kernel.
- Trust boundary: the adapter validates before write; knitknot enforces
  storage invariants only.

## Import and export

| Format | Direction | Lossless | Notes |
| --- | --- | --- | --- |
| memory-bundle (new importer) | Import | Yes | `memory_id`-keyed upsert; shape in [BUNDLE.md](BUNDLE.md) |
| Parquet | Export | Yes | nodes+edges incl. embeddings → indexer/Jupyter |
| DOT/JSON | Export | Yes | lineage visualization, debugging |

## Extension registration

A `memory` extension would register:

- Verbs: `child-of`, `similar-to`, `supersedes`, `tagged` (target labels + `MatchOn`)
- Importers: `memory-bundle` format
- Exporters: none new (generic Parquet/DOT/JSON suffice)

No new registry methods are needed — the existing
`Extension`/`Registry` contract covers this (validated by the
threat-intelligence and dfir extensions).

## Provided slots vs injected slots

What **knitknot provides** (AxiomFlow consumes, never reimplements):

- `storage.StorageEngine` implementations (`inmem` ephemeral, `bbolt`
  durable single-file) — pick per deployment
- Extension registry: verbs, importers, exporters
- Append-only versioning (`VersionedStorage`: snapshots, history, events)
- Subgraph scoping (one subgraph per user)
- DSL retrieval: `Find` / `Has` / `Where` (incl. time-aware `>`/`<`) /
  `Follow` / `Reach` (bounded, direction-aware) / `Limit`
- Export: Parquet (indexer feed), DOT/JSON (inspection)

What **AxiomFlow injects** (knitknot never implements):

- `Embedder` — embedding computation (external service/model)
- `LLMInvoker` — summaries, topic extraction (external)
- **Vector index + similarity** — out-of-band indexer engine; writes matches
  back as `similar-to` edges (`score`, `model`, timestamp) keyed by knitknot
  node ID, or as an importable results file
- **Clustering/refactoring policy** — threshold, topic caps (their
  `RefactorPolicy`); knitknot only persists the rewired tree
- **Syscall wiring** — translating runtime calls into store/query calls
- **Time source conventions** — epoch vs RFC3339 discipline for `created_at`

## Storage backend dependent capabilities

There is no single capability list — what "knitknot supports" depends on the
backend and its companions:

| Capability | inmem + gob | bolt | external payload store | external vector indexer |
| --- | --- | --- | --- | --- |
| Graph structure (nodes/edges/labels) | native | native | n/a | n/a |
| Append-only version history | native | native | n/a | n/a |
| Crash-safe persistence | file snapshot on explicit save (no WAL) | yes (CoW) | yes (store-dependent) | n/a |
| Single-file portability | yes (`.gob`) | yes (`.db`) | n/a | n/a |
| Selective reads (no full decode) | no — whole-graph gob decode | yes — B+tree point reads, incl. `blobs` | yes | yes |
| Small vectors inline (≤ 4 KiB) | native | native | n/a | n/a |
| Large vectors / ANN search | no (refs only) | no (refs only) | no | native |
| Content-addressed blobs | no — inline in file | yes (`blobs` bucket, proposed) | native | n/a |
| Time-range filtering (epoch/RFC3339) | native | native | n/a | n/a |
| Tag / array membership | no — materialize `tagged` edges or filter in Go | same | depends | n/a |
| Full-text search | no | no | depends (e.g. SQLite FTS) | n/a |
| Multi-writer concurrency | no (single-writer) | single writer, concurrent readers | depends | n/a |
| Subgraph scoping | native | native | n/a | n/a |
| Parquet export | native (engine-level) | native (engine-level) | n/a | n/a |

Notes: `sqlite` is a reserved scheme with no backend built; `blobs`-bucket
and size-rule entries marked proposed are not yet implemented. Formerly this
section listed these as absolute gaps; they are backend-scoped instead —
nothing here is missing *everywhere*.

## Security and access control

None implemented. No per-user ACLs, no marking enforcement, no audit log
beyond version history. Recommended deployment: one store file per user
(`bbolt`), filesystem protections. Do not present this as access-controlled
storage.

## Testing strategy

- Adapter contract tests against `inmem` (CRUD, lineage, versioning,
  subgraph isolation)
- `memory-bundle` fixture round trips (incl. `similar-to` write-back)
- Cosine brute-force correctness against a known small matrix (adapter-level)
- Concurrency: single-writer + concurrent readers on bbolt
- Performance fixture: 10k nodes/user retrieval latencies (the spec's own bar)

## Delivery stages

1. **Adapter**: `MemoryStore`-shaped layer on knitknot (CRUD, lineage via
   `Reach`, versioning passthrough), inmem-backed, mocked embedder/LLM.
2. **Retrieval**: type/depth/time filters via DSL; brute-force cosine in Go.
3. **Persistence + refactor**: bbolt backend; lineage rewrites via edge
   Delete+Add; `similar-to` write-back path.
4. **Indexer handoff**: Parquet export/import loop with the external engine.

## Definition of done

- Store/retrieve/version a per-user forest through the adapter with zero
  LLM/vector calls inside knitknot's code paths.
- `GetLineage` returns correct root paths; depth-bounded retrieval respects
  `MaxDepth`; time-range filters work on epoch/RFC3339 props.
- 10k-node/user retrieval and brute-force similarity within the latency
  budget the AxiomFlow side sets.
- `go test ./...` green; no new cgo/ML dependencies in knitknot.

## Open decisions

- Node label for tactic-like grouping nodes if memory adopts topic taxonomies
  (`topic_cluster` suffices for v1; revisit if shared vocabularies emerge).
- `similar-to` edges (decided: edges keep traversal; `score`/`model`/
  `observed_at` ride on the edge).
- `tagged` edges (decided: the DSL-queryable form of `tags`; Go-side
  filtering remains a fallback for ad-hoc needs).
- `memory-bundle` shape (decided: [BUNDLE.md](BUNDLE.md)).
- Whether per-user subgraph vs per-user file is the isolation unit.
