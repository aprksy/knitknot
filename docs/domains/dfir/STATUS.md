# DFIR — Current State

> **Living document.** A point-in-time snapshot of what users can rely on
> from the DFIR case × CTI correlation domain in this build. Updated as the
> domain changes. Last updated: 2026-10-08.
>
> Design intent lives in [README.md](README.md); this file describes
> behavior as it is. If a statement here disagrees with reality, reality
> wins and this file is wrong — file an issue or send a PR.

The domain is **experimental**: it works end to end and is tested, but it
is deliberately narrow. It correlates a case observable list against a CTI
graph. It is not a DFIR platform.

---

## What works

```bash
knitknot correlate --observables case.csv -f cti.gob
knitknot correlate --observables case.csv -f cti.gob --workspace case.gob
```

Covered by `extensions/dfir/*_test.go`:

- **STIX pattern observable extraction** — hash (SHA-256/SHA-1/MD5), domain,
  ipv4, ipv6, url, file-name. Tolerant of quote/whitespace variation,
  `AND`/`OR`/`FOLLOWEDBY`, grouping parens, and `WITHIN`/`START`/`STOP`
  qualifiers. Unsupported operators (`LIKE`/`MATCHES`/`ISSUBSET`, `!=`/`<=`/`>=`)
  and unknown object paths are skipped, never matched.
- **Case CSV input** — `type,value[,context]`, case-insensitive type
  aliases (observables plus `technique`), blank-line skip, and row-naming
  errors on bad input.
- **Correlation** — one `Match` per case observable that hits, walking
  `indicator —indicates→ malware —uses→ attack-pattern` and
  `campaign —attributed-to→ actor` (campaign found through the incoming
  `uses` edge). Ref lists deduped; a miss yields no entry.
- **Canonical matching** — domains lowercased with one trailing dot
  stripped; hashes lowercased; exact match thereafter.
- **Workspace projection** — `--workspace` writes a thin, self-contained
  graph (`.gob` path or `bolt:<path>`): observable nodes
  (`label=observable`), matched indicators, the reachable context, and the
  inducing/context edges. Source graph is never mutated; `based-on` links
  make `Has('based-on', <value>)` resolve.
- **Full observable layer** — `--all-observables` materializes observables
  for *every* observable-bearing indicator, not just matched ones (opt-in;
  deduped by `Observable.Key()`), so the layer can be exported and
  cluster-analyzed.
- **Coverage (evidence × pattern)** `[updated 2026-10-08]` —
  `coverage --observables case.csv -f cti.gob --target <label>:<name>`
  reports, per label, how much of the target's CTI neighbourhood the evidence
  **directly** covers: `indicator` (observable match) and `attack-pattern`
  (`technique` evidence), with matched/unmatched lists. Everything
  reachable-but-not-direct (malware/campaign/actor) is **inferred context**,
  reported separately and never counted, so a single matched IOC cannot
  inflate the score. `Total == 0` renders `n/a`, not 100% (ADR 0006).
- **Ordered coverage** `[updated 2026-10-08]` — on the `attack-pattern`
  dimension, coverage is also reported **in sequence**: covered techniques are
  ordered by evidence order (an optional `order` column, else row order) and
  compared to the expected tactic rank; `ordered = LIS/N` (longest
  non-decreasing subsequence over the expected order; ADR 0007). Rendered as
  `ordered LIS/N` with the expected vs actual sequence; `n/a` when the pattern
  has no ranks (`import --tactics`) or there is no order. `N` is always shown
  — one technique is trivially "in order".
- **Downstream tooling** — a workspace queries like any graph and exports
  to Parquet (and STIX for the nodes carrying `stix_id`).

---

## What users should not expect

### No temporal model

There is no event time, no time-range filter, and no point-in-time query.
The DSL's `>`/`<` coerce numbers only, so date-range `Where` never matches.
Timeline-shaped work belongs in the DFIR tool that produced the observables
or in Jupyter.

### Only indicator patterns are consulted

Correlation indexes observables found in `indicator` patterns. Observables
that exist only as standalone SCO nodes (`domain-name`, `file`, …) or in
`observed-data` are not consulted. A CTI bundle whose observables are not
expressed in indicator patterns will not match.

### Only the main chain is walked

`indicates`, `uses`, and `attributed-to` are traversed. `communicates-with`,
`targets`, `based-on`, `derived-from`, and `object-ref` are not.

### Exact match only

Matching is exact after canonicalization. No substring, fuzzy, or
wildcard matching; no hash-prefix matching.

### Single case per workspace

Multiple cases are not separated beyond `source_feed`. Building two cases
into one workspace merges their observable nodes by canonical value.

### Not a DFIR domain model, not evidence-grade

No hosts, users, processes, or artifacts; no markings enforcement; no
chain-of-custody. Case observables may be sensitive — the workspace file
inherits filesystem protections and nothing more.

---

## Surface

| Piece | Location | Notes |
| --- | --- | --- |
| Observable model + canonicalization | `extensions/dfir/observable.go` | `Key()` = type + NUL + canonical value |
| Pattern extractor | `extensions/dfir/extract.go` | stdlib only; never panics; skips unsupported |
| Case CSV input | `extensions/dfir/case.go` | aliases, optional context, row-naming errors |
| Correlation | `extensions/dfir/correlate.go` | read-side; reuses extractor |
| Workspace projection | `extensions/dfir/workspace.go` | thin; source unmutated |
| STIX vocabulary (local) | `extensions/dfir/vocab.go` | declared locally to avoid a domain→domain import |
| CLI | `cmd/correlate.go` | `--observables`/`-O`, `--workspace`, `--source`, `--format` |

---

## Continuity — open items

1. **Multi-case separation** — first-class per-case tagging beyond
   `source_feed` when more than one case shares a workspace.
2. **Broader traversal** — include `communicates-with` / `targets` /
   `based-on` context edges if real CTI data shows them mattering.
3. **SCO-node observables** — consult observables held as standalone SCO
   nodes, not just indicator patterns.

---

## How to update this file

When the domain gains a capability, fixes a limitation, or changes a
behavior: update the relevant section here in the same PR. Keep the
blueprint ([README.md](README.md)) as the design record and this file as
the behavior record; don't let them contradict each other.
