# ADR 0009 — Write statements in the DSL

- **Status:** accepted (design) — not yet implemented (stages W1–W4 below)
- **Date:** 2026-10-22
- **Deciders:** aprksy (owner)

## Context

knitknot reads through a real grammar (`pkg/dsl`: `Find`/`Has`/`Where`/
`Limit`, parsed from a string) but writes through two side doors: REPL
commands (`ADDNODE`, `CONNECT`, `UPDATE`/`DELETE`, `DEFINE`) parsed by
ad-hoc `strings.Fields` code in `cmd/`, and direct Go API calls. There is
no string-encoded write path a non-interactive consumer can use.

Two consumers now need one. knitgen's planned HTTP server must send writes
as strings over the wire (`strategy-datagen.md` §5 assumes an `insert`
that does not exist), and axiomflow needs scriptable writes it can drive
without linking Go code. The strategy doc deferred this (§10, 2026-10-22);
the owner has since reversed the deferral: without write-DSL, both
consumers are stuck.

## Decision

Promote the existing REPL verbs into the DSL grammar — do not invent new
ones:

1. **Syntax = the REPL verbs, canonicalized.** `ADDNODE Label k=v ...`,
   `CONNECT <a> --rel--> <b>`, `UPDATE`/`DELETE NODE|EDGE`, `DEFINE`
   become grammar statements in `pkg/dsl`, with the same spelling users
   already type. No Cypher-style `INSERT`/`CREATE`: zero new syntax to
   learn, and every existing script keeps working.
2. **One statement per program** (same as queries today). Multi-statement
   atomic scripts are recorded as a follow-up, not built now.
3. **Writes return IDs.** A generator cannot wire relationships without
   them (create customer → get ID → create order). The result shape is
   contract from day one.
4. **One executor entry point** (`dsl.Execute`) beside the query path,
   driving the existing `GraphEngine` mutation methods. No new query or
   storage logic.
5. **REPL cutover with parity proof.** The REPL's write commands are
   re-routed through the new parser; every existing `exec_*_test.go` must
   pass unchanged; the ad-hoc parsers are then deleted.
6. **Wire format falls out for free.** The future HTTP server (still
   unapproved) sends the same DSL strings — no separate write protocol.

Rejected permanently: putting the REPL's `strings.Fields` parsing on the
wire (not a grammar — freezing it would be regrettable), and JSON-write
envelopes as the primary path (kept only as a fallback if Phase 1 scoping
demands it).

## Consequences

- The write statements become public API the moment they parse — grammar
  additions after `v1.0.0` are breaking changes. We are still `v0.x`, so
  the window for shaping them is now.
- Fail-before-mutate semantics (cf. the memory importer: dangling refs
  abort, unknown labels rejected) apply to DSL writes.
- Strategy §10's deferral is superseded by this ADR; its design
  constraints (JSON-writes default, REPL-parsing rejection, pre-v1
  revisit) are adopted as requirements here.

## Out of scope

- Multi-statement transactions / scripts.
- Auth, TLS, or any server work (separate, unapproved).
- New storage or query-engine behaviour.

## Delivery stages

- **W1** — parser + AST for write statements; golden tests on parse output.
- **W2** — executor + `dsl.Execute`; create→query round-trip and error tests.
- **W3** — REPL cutover; parity tests green; delete ad-hoc parsers.
- **W4** — docs (`dsl-syntax.md`, ROADMAP/CHANGELOG, strategy §10 flip).

## Risks

- **Grammar freeze** — mitigated by minimal surface (existing verbs only)
  and by doing this while still `v0.x`.
- **REPL parity regressions** — the existing `exec_*` tests are the net;
  W3 fails if any of them changes behaviour.
- **ID-shape lock-in** — the write result format is designed once, in W2,
  with knitgen's and axiomflow's needs on the table.
