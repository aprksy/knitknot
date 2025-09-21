# KnitKnot Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- 

### Changed
- 

### Fixed
- 

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