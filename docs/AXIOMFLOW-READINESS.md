# axiomflow Readiness — knitknot v0.1.0

Date: 2026-10-19. Branch base: `main` at `cae602b`
(Merge PR #43, 2026-10-18). Tag: `v0.1.0` on that commit.

## Status

`main` builds clean, all tests pass, `go vet` and `gofmt` are clean
(evidence below). The `omos/memory-contract` lane is fully merged into
`main`: `git log main..omos/memory-contract` is empty and the merge-base
is the lane tip `ede43a4`. No uncommitted changes at tag time.

## What axiomflow can import

Module: `github.com/aprksy/knitknot`. Pin: `require github.com/aprksy/knitknot v0.1.0`.
Per GOVERNANCE.md Public API surface:

| Package | Role |
|---|---|
| `extensions/memory` | Memory domain layer: vocabulary consts, bundle types (`Bundle`, `BundleNode`, `BundleEdge`, `ParseBundle`), `MemoryImporter` (`NewMemoryImporter`, `Import`), `Extension` (`New`) |
| `pkg/ports/types` | Core types: `Node`, `Edge`, `Snapshot`, `Event`, `Subgraph`, `Verb`, `VerbRegistry` |
| `pkg/ports/storage` | `Backend` interface, `Selector` |
| `pkg/ports/query` | `QueryEngine`, `ResultSet`, `Filter`, `Direction`, `PatternNode`, `PatternEdge` |
| `pkg/graph` | `GraphEngine` kernel primitives |
| `pkg/query` | `DefaultQueryEngine` — `Find`, `Has`, `Where` (time-aware `>`/`<`, RFC3339 + epoch), `Follow`, `Reach`, `Limit` |
| `pkg/dsl` | Query builder |
| `pkg/storage/bolt` / `pkg/storage/inmem` | Backends (`Open`, `OpenNoSync`, `OpenTemp` / `New`) |
| `pkg/exporter/parquet` | `ExportToParquet` |
| `extensions` | Plugin contract (`Extension`, `Importer`, `Exporter`, `Registry`, `NewRegistry`) |

Not importable (may change any time): `cmd/`, `extensions/cti`,
`extensions/dfir`, `.slim/`. This repo never calls an LLM, computes
embeddings, or runs similarity math — embeddings/LLM reasoning are
injected from outside (see `docs/domains/memory/README.md`).

## Tags changed

- **Deleted `V0.1.0`** (uppercase V, pointed at stale `777d575` from
  2025-09-21 "add change log", far behind current `main`): `git tag -d V0.1.0`
  + `git push origin :refs/tags/V0.1.0`. Reason: Go tooling rejects
  uppercase-V tags — `go get` only resolves lowercase `v` semver, and the
  old tag pinned a stale commit, so any consumer resolving it would get
  the wrong code. Tag delete/push does not touch `main` history, so the
  never-push-to-main rule is unaffected.
- **Created `v0.1.0`** (lowercase, annotated) on current `main` HEAD
  `cae602b`, pushed to origin. Message covers: memory contract spec,
  memory extension (vocab/bundle/importer), time-aware query filters, CI,
  GOVERNANCE.md. Verified: `git ls-remote --tags origin` shows
  `refs/tags/v0.1.0` → `cae602b`.

## Verification evidence (real output, 2026-10-19, on `main` = `cae602b`)

- `git status --porcelain=v1` → empty (clean); `git rev-parse origin/main`
  = `cae602b` = local `main` (in sync after `git merge --ff-only origin/main`).
- `make build` → exit 0 (`go build -ldflags ... -o knitknot .`).
  Note: the pre-tag build stamped `version=V0.1.0` via `git describe`
  picking up the broken tag — another symptom of the bad tag, now gone.
- `go test -count=1 ./...` → exit 0. All packages `ok`
  (cmd, extensions, extensions/cti, extensions/dfir, extensions/memory,
  pkg/dsl, pkg/exporter/dot, pkg/exporter/parquet, pkg/graph,
  pkg/ports/storage, pkg/query, pkg/storage/bolt, pkg/storage/inmem);
  `[no test files]` only for root, pkg/ports/query, pkg/ports/types,
  pkg/storage/file.
- `go vet ./...` → exit 0, no findings.
- `gofmt -l` over tracked `*.go` (excluding `.slim/`) → empty, exit 0.
- `make test` / `make lint` → **no rule** (exit 2) — the Makefile defines
  only `build` and `install`. Equivalents run directly: `go test ./...`
  and `go vet` + `gofmt` check (see Known gaps).

## Known gaps

1. **Makefile lacks `test` / `lint` targets** although GOVERNANCE.md
   reserves them (`make test` = `go test ./...`, `make lint` = `go vet` +
   `gofmt` check). `make test` / `make lint` currently fail with
   "No rule to make target". Suggested follow-up: add the two aggregate
   targets (one concern, one PR).
2. **CI `gofmt` scans `.slim/`**: `.github/workflows/ci.yml` runs
   `gofmt -l .`, which descends into `.slim/worktrees/` (full repo copies)
   and would double-report issues. GOVERNANCE.md says tree scans must skip
   `.slim/`. Currently green, but the exclusion should be added before it
   causes a false red.
3. **`v0.x` is pre-stable**: anything under `v0` may still change per the
   versioning note; `v1.0.0` declares the public surface stable. axiomflow
   should pin the exact tag.
4. Machine name for the "verified on" convention was not established in
   this session, so verification is reported as run in this checkout
   rather than attributed to a named host.
