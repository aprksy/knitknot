# ADR 0010 — DSL-over-HTTP server (AD-1–AD-4)

- **Status:** accepted (design) — no server code yet; gates issue #57
- **Date:** 2026-10-23
- **Deciders:** aprksy (owner)

## Context

The write DSL is merged (ADR 0009, #48): `dsl.ParseProgram` parses reads
and writes, `GraphEngine.Execute` runs both, the REPL is unified on the
grammar. knitgen's HTTP client targets `POST /v1/execute` — but nothing
listens. The data-generator strategy (`docs/strategy-datagen.md` §5)
requires one wire endpoint before Phase 1 code lands, and its §9
definition of done requires this ADR first.

## Decision

### AD-1 — The DSL is the API, not a REST verb surface

Expose **one** endpoint taking a DSL program, not `POST /nodes`,
`GET /nodes/:id`, etc.:

```
POST /v1/execute   {"dsl": "ADDNODE Person name='Ada'\nFind('Person')"}
                   → {"ok": true, "results": [...]}
```

A REST surface would freeze dozens of shapes the moment a consumer
touched them — and frozen-forever is the rule for anything another
project imports. One endpoint taking a DSL string freezes almost
nothing and inherits the DSL's own versioning. Both halves already
exist behind one entry point (`GraphEngine.Execute`, ADR 0009), so the
handler is thin: parse → execute → serialize.

### AD-2 — The server lives in `cmd/`, not `pkg/`

`cmd/serve` opens a storage backend and serves the endpoint. `cmd/` is
explicitly not importable, so the server adds no public surface.
Promoting it to an embeddable package later is a separate, reviewed
decision — never a side effect of this work.

### AD-3 — No HTTP framework

Go's `net/http` with `ServeMux` is sufficient for one endpoint. A router
framework would expand the dependency surface for no benefit at this size.

### AD-4 — Export formats extend the existing exporter package

`pkg/exporter/` already holds `dot` and `parquet`. CSV and SQL (strategy
Phase 2, separate issue) are added the same way, behind the same
interface shape, so the package stays coherent. Recorded here because
the strategy gates all four decisions on this ADR — not because Phase 1
builds them.

## Security posture

The endpoint is remote graph mutation. Phase 1 binds **loopback by
default** and documents it; authentication, TLS, multi-tenancy, and rate
limits are explicitly deferred, not forgotten. Any remote or multi-user
use requires auth first — that is a hard constraint, not a TODO.

## Consequences

- The wire envelope (`POST /v1/execute`, `{"dsl"}`, `{"ok","results"}`) is
  owned by knitknot. knitgen follows; compatible versions pin together.
- The DSL grammar is now load-bearing over the network too — grammar
  changes stay additive until `v1.0.0`, same discipline as ADR 0009.
- Phase 1 verification bar (carried from the strategy): a `curl`
  transcript showing an insert followed by a query returning the
  inserted node — content asserted, never just HTTP 200 (false-green
  rule).

## Out of scope

- Auth, TLS, multi-tenancy, rate limits (deferred with the constraint above).
- CSV/SQL exporters (Phase 2).
- Python client, Jupyter integration, LLM compiler, statistics (outside
  this repo; rule 10 forbids the last two here outright).

## References

- Issue #57 (Phase 1 server — this ADR gates it; the issue stays open)
- `docs/strategy-datagen.md` §§2, 5, 9
- `docs/adr/0009-write-dsl.md`
