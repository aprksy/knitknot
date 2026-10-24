# ADR 0011 — Batch program ingestion (no atomicity promise)

- **Status:** accepted (design) — no code yet; gates the #70 implementation
- **Date:** 2026-10-24
- **Deciders:** aprksy (owner)

## Context

knitgen's schema-first path emits multi-thousand-line DSL programs and
executes them as one `POST /v1/execute` per statement — the only shape
the server accepts (ADR 0010, AD-1). Measured live: 3,179 statements,
all accepted, readback exact — but no server atomicity (lines after a
runtime failure do not run, earlier ones stay), and entities×years
fan-out reaches tens of thousands of statements quickly (issue #70).

ADR 0009 explicitly deferred this ("multi-statement atomic scripts are
recorded as a follow-up"). This ADR settles it.

## Decision

**Option B: a batch endpoint with per-statement results and no
atomicity promise.** Rationale:

- knitgen already validates locally (`validate_dsl`) and stops on first
  failure client-side; its `execute_program` maps live ids, so resuming
  past a failure does not depend on predicted numbering. The failure
  modes validation cannot catch (server-side, e.g. a race on shared
  state) are rare against a validated stream — and the cost of insuring
  them (Option A) exceeds the loss.
- **Option A (all-or-nothing), priced and rejected for now:** bolt
  already has real transactions (`Storage.Batch`), but inmem — the
  default serve path — has none. Atomic batches would need new
  transaction machinery for inmem, or an honest two-tier guarantee
  (atomic on bolt, best-effort on inmem). Revisit only if partial
  failures prove unrecoverable in practice; bolt's `Batch` is the
  vehicle when that day comes.

### Wire shape (new endpoint, old one frozen)

`POST /v1/execute_batch   {"program": "<statements, newline-delimited>"}`
→ `{"ok": true|false, "executed": n, "results": [<per-statement>]}`
where each per-statement result carries the same fields as
`/v1/execute` (`results` rows or `node_id`/`edge_id`/`verb`, or
`error`). Execution stops at the first failing statement; statements
after it never run; `executed` counts the ones that did. `/v1/execute`
keeps its exact current shape — existing clients are untouched.

### Grammar: newline delimits, strings exempt

Statements are separated by newlines; a quoted string may still span
lines (the lexer already permits it). The parser learns statement
boundaries from line position, not from a new terminator token — no
DSL syntax change, so single-statement programs parse identically.
Trailing garbage after the last statement is an error, as today.

## Consequences

- Throughput scales with one round-trip instead of N; correctness
  properties (fail-before-mutate per statement, ID contract) are
  unchanged — the batcher reuses `GraphEngine.Execute` per statement.
- Partial failure is observable (`executed` + per-statement `error`),
  never silent — the false-green rule applies: tests must assert the
  failing statement's index and that later statements did not run.
- The no-atomicity promise is documented in `docs/serve.md` when the
  endpoint lands (not in this ADR alone).

## Verification bar (for the implementation PR)

- The #70 HSE program at 10x through one batch call, readback exact.
- A batch with a mid-stream dangling ref: `ok:false`, `executed`
  correct, failing statement's error present, later statements absent.
- `make build && make test && make lint` green; goldens for the
  multi-statement parse (including a quoted multiline string).

## Out of scope

- Atomic batches (revisit trigger above).
- Streaming/chunked ingestion, COPY-style bulk protocols.
- Serving export over HTTP; CSV/SQL import (#38) is adjacent, not this.

## References

- Issue #70 (this ADR advances it; implementation closes it)
- ADR 0009 (write-DSL contract + entry point), ADR 0010 (AD-1, server)
- `docs/strategy-datagen.md` §9 (ADR-before-server-code pattern)
