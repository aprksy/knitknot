# ADR 0007 — Ordered coverage (sequence-aware similarity)

- **Status:** accepted — decisions confirmed; stages 1–3 to implement
- **Date:** 2026-10-08
- **Deciders:** knitknot maintainers

## Context

Coverage (ADR 0006) is **set overlap**: which of a target's pattern elements
the evidence directly matches. It ignores **order**. Ordered coverage adds
strictness: the evidenced elements should also appear in the **expected
relative order** — the "semi-structural" property.

Order has two sides:

- **Pattern side** — tactic rank (ADR 0005). Coarse: it orders techniques by
  kill-chain phase, not by within-phase sequence.
- **Evidence side** — a logical order (analyst/Jupyter supplies it; row order
  is the default). Wall-clock is deliberately not required (window/clock
  unreliability), consistent with the logical-time discussion.

**Only the `attack-pattern` dimension is orderable** (it carries tactic rank).
`indicator` has no rank, so ordered coverage does not apply to it.

## Decision

Extend coverage with an **ordered** view, computed only when both sides carry
order:

1. **Evidence order** — the case input gains an optional `order` column
   (numeric; ascending = earlier). Absent → the **row order** is the sequence.
   Header-named, so existing `type,value[,context]` files are unaffected.
2. **Technique ranks** — for each covered `attack-pattern`, resolve its tactic
   `rank` (ADR 0005). Multi-tactic → take the **minimum** rank (earliest
   phase). Techniques with no rank are excluded from ordering (still counted
   in set coverage).
3. **Metric** — order the covered, ranked techniques by evidence order to get
   a rank sequence `r₁…r_N`, then:
   `ordered_coverage = LIS(r₁…r_N) / N`
   the longest non-decreasing subsequence over the expected order.
   Fully in tactic order → `1.0`; fully reversed → `1/N`.
4. **Report** — surface both `coverage` (set) and `ordered_coverage`
   (sequence) for the `attack-pattern` dimension, plus the expected vs actual
   sequence so a human can see the violation.

## Semantics

- **Coarse by construction.** Ordering is tactic-level; two techniques in the
  same tactic are treated as unordered (non-decreasing LIS, so equal ranks are
  not violations).
- **Gaps tolerated.** LIS is a subsequence metric — missing elements
  (unseen windows) don't break order.
- **Prerequisite.** Ordered coverage requires a tactic-materialized pattern
  (`import --tactics`, ADR 0005). Without ranks it is reported as `n/a`, never
  fabricated.
- **Not probabilistic.** It is a coverage/order statement, not a confidence.

## Out of scope

- Fine-grained technique sequencing (not present in STIX).
- Probabilistic / learned similarity (Jupyter).
- Ordering the `indicator` dimension (no rank exists).

## Delivery stages

1. `order` column on the case input (optional; row-order fallback).
2. Rank resolution for covered techniques (via `has-tactic`).
3. LIS-based `ordered_coverage` + report (expected vs actual sequence).

## Risks

- **Sparse evidence inflates order.** A single evidenced technique is
  trivially "in order" (LIS = N = 1). Ordered coverage is only meaningful with
  enough covered elements — report `N` alongside so it is not over-read.
- **Tactic order is approximate** — real attacks loop phases; a strict read
  can produce false "out of order".
- **Ambiguous order keys** — ties/row-order defaults can be accidental; the
  `order` column is the honest path.

## Open decisions (resolved for v1)

- **LIS strictness for equal ranks:** non-decreasing (confirmed) — two
  techniques in the same tactic are not a violation.
- **Per-pair inversions:** **not in v1** — the *expected vs actual sequence*
  is the explainability (a human can see the violation directly); a numeric
  inversion count is a later addition if needed.
- **Reporting:** ordered coverage is **always reported**, rendering `n/a` when
  ranks or evidence order are unavailable (confirmed) — not behind a flag.
