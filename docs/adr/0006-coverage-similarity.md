# ADR 0006 — Coverage: evidence × pattern similarity

- **Status:** accepted — stages 1–3 shipped (`coverage` CLI: pattern + direct evidence + report); ordered coverage = stage 4, separate
- **Date:** 2026-10-08
- **Deciders:** knitknot maintainers

## Context

For slow / low-and-slow hunting, the core signal is: **how much of a known
adversary's pattern have we actually seen?** Two layers:

- **Coverage** — set overlap between accumulated evidence and a reference
  pattern. General; most cases need it.
- **Ordered coverage** — sequence-aware, strictly stronger. An add-on.

knitknot supports coverage (and, later, ordered coverage); probabilistic /
learned similarity stays in Jupyter. Prerequisites are shipped: multi-hop
traversal (ADR 0004) and tactic ranks (ADR 0005).

## The critical distinction: direct vs inferred

A **single** matched IOC reaches the entire adversary chain
(`indicator →indicates→ malware ←uses← campaign →attributed-to→ actor`). If
coverage counted everything *reachable*, one match would report ~100% — noise.

Therefore: **coverage counts only what evidence directly maps to.**
Everything reachable-but-not-directly-evidenced is **context**, reported
separately, never counted.

## Decision — the coverage model

**Pattern (target).** Given a target CTI reference node (actor / campaign /
malware / technique), collect its CTI neighborhood (multi-hop, both
directions) grouped by label:
`Pattern_L` for `L ∈ {indicator, attack-pattern, malware, tool, …}`.

**Direct evidence.** Evidence entities that map to a CTI label by direct,
checkable equality:

| Evidence entity | Maps to label | Match |
| --- | --- | --- |
| observable (domain/ip/hash/url/file-name) | `indicator` | canonical pattern-observable equality |
| `technique` (ATT&CK id or name) | `attack-pattern` | `name` (or `external_id` when resolvable) |
| *(extensible)* `malware`/`tool` (name) | `malware`/`tool` | `name` |

**Coverage per label:**
`Coverage_L = |Pattern_L ∩ DirectEvidence_L| / |Pattern_L|` (1.0 when
`|Pattern_L| = 0` is reported as *n/a*, not 100%).

**Output:** per-label fractions + matched/unmatched lists + optional
weighted overall; plus an **inferred context** section (reachable via
matches, explicitly *not* counted).

**Ordered coverage (follow-on, separate decision):** apply the same
computation but require the covered elements to appear in the expected
relative order — tactic `rank` (ADR 0005) on the pattern side, evidence
logical order on the other. Strictness is opt-in.

## Evidence model extension

The case CSV (`type,value[,context]`) gains `technique` (and optionally
`malware`/`tool`). Existing observable types are unchanged. This is what
makes coverage work for **MITRE ATT&CK** references (which carry no
indicators): the `attack-pattern` dimension is populated by `technique`
evidence an analyst/Jupyter supplies.

## CLI

```
knitknot coverage --observables case.csv -f cti.gob --target <label>:<name> [--format text|json]
```

`--target` selects one reference node (a missing/ambiguous target errors).

## Honest limits

- **Only directly-matchable layers count.** Behavioral technique coverage
  requires the evidence to carry technique annotations (analyst/Jupyter
  supplies them); knitknot does not infer a technique from behavior.
- **ATT&CK bundles have no indicators** → the `indicator` dimension is empty
  there; `attack-pattern` is the useful dimension.
- **Pattern completeness depends on the bundle.** Coverage is relative to
  what is in the reference graph, not to ground truth.
- **`external_id` resolution** (technique `T####`) reads
  `external_references`, an array prop; v1 matches techniques by `name` and
  treats id-matching as best-effort.
- **Not confidence, not attribution.** "Covers 3/6 techniques" ≠ "it is
  actor X" — many actors share techniques.

## Delivery stages

1. **Pattern extraction** — target neighborhood per label (both directions).
2. **Direct-evidence mapping** — observables→indicators, techniques→
   attack-patterns (extend the case input).
3. **Coverage computation + report + CLI.**
4. **Ordered coverage** — separate ADR once 1–3 land.

## Open decisions (resolved for v1)

- **Target selector:** `--target <label>:<name>` (errors on missing/ambiguous).
- **Dimensions v1:** `indicator` + `attack-pattern` (the directly-evidenced
  layers). `malware`/`tool` direct-evidence types are a trivial extension,
  deferred.
- **Overall score:** report per-label fractions only; **no weighted
  aggregate** in v1 (consumers weight).
- **Inferred context:** a collapsed count in `text`; the full list in `json`.
