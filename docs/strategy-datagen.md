# STRATEGY — knitknot as a graph engine behind LLM-assisted data generation

**Status:** accepted — owner approved by merging the server (PR #60).
Phase 1 shipped; Phases 2+ pending. This document is now a record
plus the remaining plan, not a gate.
**Author:** Hermes (agent) on behalf of aprksy
**Date:** 2026-10-21
**Scope:** strategy only. No implementation in this document.

---

## 1. The idea in one paragraph

A user opens a Jupyter notebook and writes a scenario in natural language
("create 3 large corporations operating in Indonesia, 50–200 employees each,
salaries roughly normal"). Jupyter calls an LLM, which compiles that scenario
into **knitknot's existing DSL** plus a stream of generated property values. The
DSL is sent to a knitknot **server** over HTTP; knitknot owns the graph —
entities, relationships, constraints, persistence — and exports the result as
CSV, SQL INSERTs, Excel, or Parquet.

The point is **promotion of knitknot**, not new statistical machinery inside it.
knitknot becomes the engine people reach for when they need relationship-correct
synthetic data.

---

## 2. The boundary (governance rule 10)

`GOVERNANCE.md`, *For AI agents*, rule 10 is binding:

> This repo never calls an LLM, never computes an embedding, never runs
> similarity math. […] Embeddings, LLM reasoning, and vector indexing are
> injected from outside.

This strategy is built to **honour** that rule, not to argue with it:

| Concern | Where it lives | Why |
|---|---|---|
| Natural language → DSL | Jupyter (Python), calls LLM | LLM reasoning stays outside the repo |
| Property values (names, addresses, prose) | Jupyter (Python), calls LLM | same |
| Graph structure, relationships, constraints | **knitknot** | this is the product |
| Persistence, query, export | **knitknot** | already exists |
| Distributions for *numeric* fields | Jupyter (numpy/scipy) | keeps statistics out of knitknot |

**Consequence:** knitknot gains a *server* and *export formats*. It gains no
LLM client, no prompt templates, no randomness policy, no faker library.

If this boundary is ever breached, the axiomflow contract breaks with it — that
is the cost, and it is the reason the boundary is written here first.

---

## 3. Why this promotes knitknot

- **The DSL becomes the selling point.** It is already mature, documented
  (`docs/dsl-syntax.md`), and simpler than SQL for graph-shaped data. Exposing
  it over HTTP turns a CLI feature into an integration surface.
- **Relationship correctness is the differentiator.** Every faker library
  generates flat rows. knitknot generates *connected* rows: an order that
  references a real customer who references a real region. That is the gap in
  the market this fills.
- **Demoability.** A notebook that goes from a sentence to a validated,
  exportable dataset is a better demo than any README.
- **Axiomflow benefits too.** The same server is how axiomflow (or any future
  consumer) can drive knitknot over a process boundary instead of linking it.

---

## 4. Scope: what is and is not in this proposal

### In scope (knitknot repository)

1. **Makefile contract compliance** — prerequisite, see §6.
2. **DSL-over-HTTP server** in `cmd/` — execute a DSL program, return results.
3. **Export formats** — CSV and SQL `INSERT` in `pkg/exporter/`.
4. **ADR** recording the DSL-as-API decision.

### Out of scope (separate repositories, later)

5. Python client / Jupyter integration.
6. LLM prompt → DSL compiler.
7. Any statistical generator.

Items 5–7 do not belong in this repo, and rule 10 forbids 6 and 7 outright.

---

## 5. Architectural decisions

### AD-1 — The DSL is the API, not a REST verb surface

Expose **one** endpoint that accepts a DSL program, rather than `POST /nodes`,
`POST /edges`, `GET /nodes/:id`, and so on.

**Why:** the DSL is already the mature, documented interface. A REST surface
would freeze dozens of shapes the moment a consumer touched them, and per the
public-API rules that freeze is permanent. One endpoint taking a DSL string
freezes almost nothing and inherits the DSL's own versioning.

**Shape (proposal, not final):**

```
POST /v1/execute   {"dsl": "insert node customer {...}\nquery ..."}
                   → {"ok": true, "results": [...]}
```

### AD-2 — The server lives in `cmd/`, not `pkg/`

`GOVERNANCE.md` states that packages under `pkg/` are a compatibility contract,
and that "do not move a package into the public set without review" — because
the moment another project imports it, the surface is frozen.

Therefore the server starts in `cmd/` (explicitly *not* importable). If a
consumer later needs to embed it, promoting it to `pkg/api` is a separate,
reviewed decision — not a side effect of this work.

### AD-3 — No new dependency on an HTTP framework

Go's `net/http` with `ServeMux` is sufficient for one endpoint. Adding a router
framework would expand the dependency surface for no benefit at this size.

### AD-4 — Export formats extend the existing exporter package

`pkg/exporter/` already holds `dot` and `parquet`. CSV and SQL are added the
same way, behind the same interface shape, so the package stays coherent.

---

## 6. Phase plan (PR-sized, one concern each)

Governance requires one concern per PR and a recorded verification in the body.
Each phase below is one PR unless stated.

### Phase 0 — Makefile contract compliance (prerequisite)

**Problem:** `Makefile` currently defines only `build` and `install`. Governing
contract requires `build`, `test`, `lint`, `run`, `clean`, with bare form as
aggregate.

**Change:** add the missing reserved targets (`test` → `go test ./...`,
`lint` → `go vet` + `gofmt` check, `run` → run CLI, `clean` → remove `knitknot`
binary), keep `build`/`install` as-is, document scoped forms if any exist.

**Why first:** "Definition of done" says `make test` and `make lint` must pass.
Until they exist, no later PR can satisfy its own definition of done. This PR
also removes a standing inconsistency between the repo and its own governance.

**Verification:** `make build && make test && make lint` on `sxi-work`, output
pasted into the PR body.

### Phase 1 — DSL-over-HTTP server (`cmd/`)

**Change:** a `serve` command that opens a storage backend and exposes one
endpoint executing a DSL program. Reuses the existing REPL execution path; no
new query logic.

**Not in this PR:** authentication, TLS, multi-tenancy, rate limits. It binds to
loopback by default and says so.

**Verification:** `make build && make test`; a `curl` transcript showing an
insert followed by a query returning the inserted node.

### Phase 2 — CSV + SQL export

**Change:** `pkg/exporter/csv` and `pkg/exporter/sql`, following the existing
exporter package conventions. Rediscovered by the CLI (`knitknot export --format
csv|sql`) and reachable over the Phase-1 endpoint.

**Verification:** `make build && make test`; a test asserting exact CSV header
and a SQL `INSERT` statement round-trip.

### Phase 3 — ADR + docs

**Change:** `docs/adr/NNNN-dsl-over-http.md` recording AD-1 through AD-4;
`docs/ROADMAP.md` entry; a short section in `docs/dsl.md` pointing at the server.

**Verification:** docs only; `make lint` still clean.

### Phase 4+ — outside this repository

Python client, Jupyter integration, LLM compiler. Each is its own repository,
reached from here only by HTTP. No PR in this repo depends on them.

---

## 7. Risks and mitigations

| Risk | Mitigation |
|---|---|
| **Boundary creep** — someone later adds an LLM call "just for convenience" | ADR states the boundary; rule 10 already forbids it; the client lives in another repo |
| **Premature API freeze** — REST shapes become contract | AD-1 (one endpoint, DSL payload), AD-2 (server in `cmd/`) |
| **Security** — a graph server with no auth | Phase 1 binds loopback by default and documents it; auth is explicitly deferred, not forgotten |
| **Scope inflation** — "just add distributions too" | §4 out-of-scope list is explicit; distributions stay in Jupyter |
| **False green** — server tests that only assert HTTP 200 | Phase 1 verification asserts a query returns the inserted node, not just a status code |

---

## 8. Machine note (governance compliance)

`GOVERNANCE.md` says work starts and is verified on **`sxi-work`**, and that
**`oci-srv` is a follower — pull only, never leads**.

At the time of writing, `sxi-work` is unreachable (SSH to `10.0.10.2:2022` times
out). This document was prepared on `oci-srv`, which means it is **prepared, not
verified**. No code should be pushed from `oci-srv` under this strategy; Phase 0
onwards begins when `sxi-work` returns, or when the owner explicitly authorises a
different machine for a given PR.

[updated 2026-10-24: the constraint above is spent — Phases 0–1 shipped
via PRs #45 and #60. It stays as the record of how this strategy started.]

---

## 9. Definition of done for this strategy

- [x] Owner approves the boundary in §2 and the phasing in §6
  (approved by merging the server, PR #60, 2026-10-24).
- [x] Phase 0 is opened as a PR and merged before Phase 1 begins (PR #45).
- [x] An ADR exists for AD-1..AD-4 before any server code lands (ADR 0010).
- [x] Phase 1 server shipped (PR #60, `Issue: 57 | closes`).
- [ ] Phase 2 exporters (CSV/SQL) — separate issue, not started.
- [ ] Phase 3 docs/ADR follow-ups; Phase 4+ outside this repository.
