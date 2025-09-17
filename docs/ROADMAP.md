# KnitKnot Roadmap

We follow [Semantic Versioning](https://semver.org): `vMAJOR.MINOR.PATCH`

During the `v0.x` phase:
- `MINOR` increments = new features
- `PATCH` = bug fixes, stability, tooling

---

## v0.1.0 — Stabilize & Scale (Q4 2025)
- [ ] Finalize MVP architecture
- [ ] Optimize core engine for 100K nodes on laptop
- [ ] Add benchmark suite + sample DB (100K nodes)
- [ ] Improve REPL UX
  - Consistent command syntax
  - Syntax highlighting
  - Multi-line input
- [ ] Test coverage ≥ 90% on core packages
- [ ] CI/CD pipeline (GitHub Actions)

---

## v0.2.0 — Performance & Query Enrichment (Q1 2026)
- [ ] Add indexing (by label, property, subgraph)
- [ ] Enrich DSL
  - `.Select("n", "v0")`
  - `.OrderBy(...)`, `.Count()`
  - `.WhereIn(...)` for lists
- [ ] Improved output
  - CSV/TSV export
  - Pretty-print paths
- [ ] Parser validation & better error messages

---

## v0.3.0 — Production Readiness (Q2 2026)
- [ ] Embedded HTTP Server (REST API)
  - `/query`, `/addnode`, `/export`
  - JSON input/output
- [ ] File format: JSON Lines (`.jsonl`) support
- [ ] Web UI preview (static assets + DOT rendering)
- [ ] Authentication hook (for future multi-user)

---

## Future Vision
### v0.5.0 — Distributed Mode Prep
- Sharding by subgraph
- Plugin system
- Change feed / events

### v1.0.0 — Stable API
- Freeze core API
- Full documentation site
- First production adopters