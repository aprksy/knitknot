# ADR 0001 — Pluggable storage backends

- **Status:** proposed
- **Date:** 2026-09-22
- **Deciders:** knitknot maintainers

## Context

KnitKnot currently has a single storage backend (`pkg/storage/inmem`) which
is in-memory, mutex-protected, and persisted via whole-graph `gob`
snapshots through `Save`/`Load`. This works well for the analyst / REPL /
embedded niche the README and STATUS.md describe: one process, one user,
small-to-medium graphs, instant iteration.

The roadmap and Stage 2 deferred items hint at needs the current backend
cannot satisfy gracefully:

- Durable, crash-safe writes for STIX bundle import (TI Stage 2)
- Unique constraints on stable IDs (TI Stage 2)
- Indexed property lookups (CTI Stage 2)
- Edge lookups by source/target/kind (general perf cliff past ~10K nodes)
- Sharing graphs between processes or with teammates without the
  `.gob` snapshot/restore dance

These are storage properties. The right abstraction is a second backend, not
new layers of ad-hoc logic on top of the existing one.

This ADR also reflects the framing discussed in chat: think of
`gob + inmem` as the laptop and a future durable backend as the desktop
(or rack-server, for one of the larger options). The user can take the
laptop anywhere; they bring the desktop when they want the full suite.
Synchronization between the two is the user's responsibility, but the
contract between them is ours.

The laptop/desktop framing is the product story. This ADR is the technical
contract behind it.

## Decision

Introduce a **pluggable storage backend** selected by a CLI flag. The
default backend is the existing `gob + inmem`; additional backends land as
separate `StorageEngine` implementations in `pkg/storage/<name>` and
register themselves with a runtime selector. No backend depends on any
other backend; backends share only the `StorageEngine` interface.

The selection surface is a single CLI flag, `--store`, with a URI-like
value:

```text
--store gob:path/to/file.gob        # default if --store is unset
--store sqlite:path/to/file.db       # future
--store mem:                         # future, ephemeral in-memory only
```

The current `-f, --file` flag is preserved and acts as a synonym for
`--store gob:<value>`. New code should prefer `--store`; existing
`-f` callers continue to work unchanged.

A new top-level command `knitknot sync --from <store-uri> --to <store-uri>`
is reserved for cross-backend synchronization. It returns
"sync not implemented yet" until a backend pair ships a merge
implementation; the command shape is locked in by this ADR.

## Backends

| Backend   | `kind`  | State            | Use case                              |
| -------- | ------- | ---------------- | ------------------------------------- |
| inmem    | `gob`   | shipped (today)  | Analyst / REPL / single-process work  |
| sqlite   | `sqlite`| future           | Durable local storage, indexed lookups|
| mem      | `mem`   | future (optional)| Ephemeral, no persistence             |

This list is expected to change. The ADR does not enumerate every backend
that will ever exist.

## Selection contract

```go
// pkg/ports/storage/backend.go (new file)
package storage

import "io"

type Backend interface {
    Kind() string                                    // "gob", "sqlite", ...
    Open(path string) (StorageEngine, error)         // open existing file
    Create(path string) (StorageEngine, error)       // create new file
    Engine() StorageEngine                          // in-process / no file
}

type Selector interface {
    Register(b Backend)
    Resolve(uri string) (Backend, error)
}
```

- `Backend.Kind()` returns the URI scheme (`"gob"`, `"sqlite"`, ...).
- `Open(path)` opens an existing file; `Create(path)` creates a new one.
- `Engine()` returns a `StorageEngine` with no file backing (e.g. fresh
  in-memory only).
- `Resolve(uri)` parses a URI like `"gob:file.gob"` and returns the
  matching registered backend, or an error if the scheme is unknown.
- The selector is a global, process-singleton; backends register
  themselves in `init()` in their own packages.
- The `gob` backend is registered by default in `pkg/storage/inmem`; it
  does not require a new entry point.
- The `StorageEngine` interface in `pkg/ports/storage/storage.go` is
  unchanged. Backends satisfy it; consumers program against it.

The URI scheme `gob` and `mem` may share the inmem backend. `sqlite`
maps to a distinct backend implementation. Backend implementations are
the only place that knows about file formats.

## Selection rule

When the user runs a command:

1. If `--store <uri>` is provided, parse it; resolve via the selector.
2. Else if `-f, --file <path>` is provided, treat as `--store gob:<path>`.
3. Else if the command is REPL or another interactive mode, default to
   `--store gob:data.gob` (current behavior).
5. Else (one-shot commands with no file), use `mem:` — ephemeral, fresh
   in-memory engine; mutations evaporate.

The current behavior — `-f` empty means "fresh empty inmem graph that
evaporates" — is preserved. The only behavior change is the addition of
the `--store` flag surface and the explicit URI semantics.

## Synchronization

`sync` is a top-level command:

```text
knitknot sync --from <uri> --to <uri> [--prune]
```

Semantics:

- Both objects are opened via the selector.
- A merge pass reads every node and edge from `--from` and upserts it into
  `--to` using the same stable-ID lookup + property merge that the
  CTI `STIXImporter` uses today (Stage 1). This keeps the merge logic
  in one place across import, sync, and any future domain importers.
- By default, `sync` is **additive**: it never deletes nodes or edges
  from `--to`. Use `--prune` to opt into destructive propagation
  (nodes/edges present in `--to` but absent from `--from` are removed).
- Property conflicts use the importer's merge rule (existing keys
  preserved unless the incoming value overrides).
- Edge endpoints are validated against the source's full node set
  before any mutation; a single unresolvable endpoint aborts the sync.
- Verbs are merged by name; existing verbs in `--to` are preserved
  unless overridden.

Until a backend pair implements the actual data transfer, `sync` returns
"sync not implemented for <kind> -> <kind>" with exit code 1. The
command is in place so its surface is fixed; users can document and
automate against it.

## Implementation order

1. **`pkg/ports/storage/backend.go` + selector.** Pure plumbing.
2. **`--store` flag** on the root command; document, do not wire
   into commands yet.
3. **`sync` command stub** that calls the selector on both URIs and
   returns the not-implemented error.
4. **`-f` / `--store` resolution rule** in `LoadGraph` / `SaveGraph`
   (cmd/graph_loader.go).
5. **Wire `--store` everywhere `-f` is used** in commands: `query`,
   `export`, `import`, `repl`. Existing `-f` callers continue to
   work; tests untouched.
7. **Each new backend lands as a separate `pkg/storage/<name>`** with
   its own `init()` registration. SQLite is the first candidate.

## What this ADR deliberately does not decide

- **The SQLite implementation itself.** That arrives as a follow-up
  ADR / PR with its own migration story, schema version, and indexing
  decisions.
- **Conflict resolution in detail.** The merge rule references the
  importer's behavior; if a different rule is needed for sync, that's
  a separate ADR.
- **Multi-user / network sync.** Out of scope. The `sync` command is
  single-process / single-machine.
- **Versioning the storage format.** Handled per-backend (already true
  for `gob` via `CurrentVersion`).
- **The REPL's `flush` command** (checkpoint in-process workspace to
  storage). Mentioned in chat as a future verb but not part of this
  ADR; will land separately if/when a workspace-style backend exists.

## Consequences

### Positive

- The path from "no second backend" to "ship SQLite" is paved but not
  committed. The seam exists; the work to fill it is incremental.
- The contract is testable without any new backend. Backend-agnostic
  tests can resolve `--store` URIs against a registered test backend
  and assert behavior across the contract.
- The CTI extension's importer becomes the rule surface for `sync`'s merge
  logic — no new merge algorithm invented in parallel.
- Domain extensions (CTI, future fraud / identity / healthcare) gain
  durability without their own per-domain persistence work.
- The `--store` flag is purely additive. No command behavior changes for
  existing callers.

### Negative

- More interface surface to maintain (`Backend`, `Selector`, future
  `Syncer` if it grows).
- The selector is process-singleton state; tests that register conflicting
  backends with the same scheme need ordering care.
- The `sync` command's contract is more opinionated than the simple
  "additive merge" the importer already does; encoding it now may
  prove too narrow later.

### Mitigation

- Keep `Backend` / `Selector` minimal. The selector does nothing
  fancier than scheme→backend lookup.
- Lock the selector behind a sync.Once or similar so registration is
  idempotent in tests.
- If `sync` semantics need more nuance, ship a follow-up ADR; do not
  silently expand the contract.

## Decision outcome

Adopt the contract. Land the three plumbing pieces (backend interface,
selector, `--store` flag, `sync` stub) in one PR. Defer actual new
backends to separate ADRs and PRs.