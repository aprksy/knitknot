# KnitKnot Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- cmd/ test suite: 43 Ginkgo specs covering dispatch routing, exec
  functions, and explain/export/sample regressions (previously zero
  coverage).
- pkg/storage/inmem concurrency + delete-cascade specs (5 specs);
  inmem coverage now ~91%.

### Changed
- Gob persistence format bumped to `knitknot/v0.2` (Subgraphs field
  became a `map[string]*Subgraph`). `data.gob` was regenerated and
  is now loadable; older `.gob` files produced before this release
  will need to be re-saved.
- `=` and `!=` filters coerce numerics like `>` and `<` (e.g.
  `Where('n.age', '=', 51)` now matches string-stored `"51"`).

### Fixed
- REPL `exit`/`quit` no longer bypass the deferred readline close
  and the autosave on a `-f` file; both run via an `errExitRepl`
  sentinel.
- `UPDATE EDGE` actually parses trailing properties now (was a
  dead-code branch that always reported `no properties to update`).
- `--explain` no longer panics on numeric or missing arguments;
  uses comma-ok assertions with arg-count guards.
- `GetEdgesIn` auto-inherit branch no longer races on the stored
  edge's `Subgraphs` map under `RLock`; deep-copies before write.
- Subgraph scoping now works: `--subgraph <name>` and the DSL
  `.In(<name>)` method both filter query candidates via
  `GetNodesIn`.
- Query filters without a `var.` prefix apply to the first node's
  var instead of being silently dropped (e.g.
  `Where('city', '=', 'Dallas')` from `docs/dsl.md` now returns
  only Dallas customers).
- `DeleteNode` cascades to every incident edge; previously left
  dangling references in `GetAllEdges`, DOT export, and saved
  files.
- `AddToSubgraph`/`RemoveFromSubgraph` now lock `s.mu`.
- `Save`/`Load` (and `runExport`) use named returns so deferred
  `Close` errors propagate instead of being swallowed.
- Node IDs are now generated via an atomic counter; removed the
  dead/broken `internal/util/idgen.go`.
- `export --format json` returns `not implemented` instead of a
  silent exit 0.
- `parseProps` joins spaced tokens onto the previous value, so
  `ADDNODE Person name=John Doe` now records `name="John Doe"`
  (was silently dropping `Doe`).
- DSL parser no longer panics on empty `p.errors`; the EOF
  branch returns a clearer `unexpected end of query` message;
  unterminated string literals are rejected as `Illegal` tokens.
- `generate-sample` without `-f` returns a usage error (was a
  bare `open : no such file`); `os.Stat` after `Save`/`Load` is
  guarded against nil-deref; `UPDATE EDGE` on a missing edge now
  reports `edge not found` (not `node not found`).

---

## [MVP] - 2025-09-17
### Added
- Fluent DSL: `Find()`, `Has()`, `Where()`, `Limit()`, `WhereEdge()`
- REPL shell with interactive query, `EXPLAIN`, and command history
- Verb registry: `DEFINE <verb> TO <Label> VIA <prop>`, `LIST VERBS`
- Data manipulation in REPL:
  - `ADDNODE Label key=value...`
  - `CONNECT A --rel prop=v--> B`
  - `UPDATE NODE|EDGE`
  - `DELETE NODE|EDGE`
- Subgraph scoping via `.In(subgraph)` and `--subgraph` flag
- Persistence: Save/load full graph state to `.gob` binary format
- Export to Graphviz DOT format for visualization
- Global `-f, --file` flag for auto-load/save workflows
- Comprehensive test suite using Ginkgo (coverage >90% on core)
- Fuzz testing for parser robustness
- Railroad diagram of DSL syntax

### Changed
- Architecture stabilized with clean separation: `GraphEngine`, `StorageEngine`, `QueryEngine`
- Internal query plan (`QueryPlan`) execution now supports real graph traversal and edge filtering
- Parser improved for better error messages and whitespace tolerance

### Fixed
- Early filtering bug in query engine (premature node elimination)
- Edge case in `Has(...)` where target label was derived from relationship name
- Case sensitivity handling in verb matching

### Removed
- Hardcoded relationship logic (replaced with extensible verb registry)

---

## [pre-MVP] - 2025-09-05
### Added
- MVP core engine with in-memory storage
- Basic fluent query builder
- Initial parser for `Find('Label')` syntax
- Simple REPL loop
- Save/load mechanism (early version)
- Initial documentation (`README`, `CONTRIBUTING`, `ARCHITECTURE`)
- GitHub Actions CI pipeline with coverage reporting

### Changed
- Project structure organized for scalability
- Interfaces defined for future pluggable backends

### Fixed
- Minor parsing issues in DSL lexer
- Memory leak in edge lookup (fixed with proper indexing)

### Removed
- Experimental JSON serialization (unstable)

---

## Future Releases (Planned)

See [ROADMAP.md](docs/ROADMAP.md) for upcoming features in `v0.2.0`, `v0.3.0`, and beyond.

---

_This project began development in August 2025._