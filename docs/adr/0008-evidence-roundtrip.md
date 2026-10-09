# ADR 0008 — Evidence round-trip and accumulation

- **Status:** accepted — decisions confirmed; stages 1–3 to implement
- **Date:** 2026-10-08
- **Deciders:** knitknot maintainers

## Context

Low-and-slow / APT activity means any single SIEM window shows only a
**fragment** of the symptoms a STIX pattern describes. The durable signal is
**accumulation across weeks/months**, not one-shot analysis.

knitknot already has the accumulation primitives: idempotent merge by stable
identity, append-only history, and the stale-reimport guard. And the coverage
family (ADR 0006/0007) plus correlation already consume a **case-entity**
input.

**The gap:** `correlate`/`coverage` are *read-side one-shots over a CSV* —
evidence is neither persisted nor accumulated, so the graph never grows a
long-term evidence layer.

Jupyter is the batch processor: it filters/annotates the (large) SIEM down to
symptom-relevant events — which also solves knitknot's scale limit — and can
supply the **logical order** and **technique annotations** coverage needs.

## Decision

Close the loop with an **evidence round-trip**:

1. **Exchange format = the existing case-entity model, not a new one.**
   CSV `type,value[,context][,order]` (today) or JSONL of the same for richer
   records. Kinds: observable types (domain/ip/hash/url/file-name) +
   `technique`. **Not** STIX `observed-data` (lossy — `object_refs` is not
   materialized into edges — and threat-intel-shaped, not SIEM-shaped).

2. **Evidence import** — `knitknot import --format evidence --source <window> -f <graph> <file>`:
   - persists each entity as an **`observable` node** (reusing the DFIR
     observable layer): `type`, `value`, `name`, `order`, `source_feed`;
   - **accumulates** across windows — the same canonical identity merges into
     one node (history appends); new identities add;
   - **idempotent** — re-importing a window does not duplicate.

3. **Coverage from the graph** — `coverage --from-graph [--source <window>|--all]`
   reads accumulated evidence nodes instead of a one-shot CSV, so the
   *accumulated* set is what gets scored.

4. **Jupyter side** — a documented producer (snippet/library, outside this
   repo): filter SIEM → emit the case-entity CSV/JSONL with a logical `order`
   and any technique annotations.

## Identity, order, accumulation

- Evidence identity = entity kind + **canonical value** (the same
  canonicalization `Observable` already applies). Techniques key by name.
- **One node per identity** (merged); `order` + `source_feed` recorded, history
  appended per observation — so "seen in window W1 and W3" is queryable.
- Cross-window ordering uses the producer's logical `order`; the ordered view
  (ADR 0007) consumes it.
- Merge reuses the kernel's semantics; the stale guard applies only where a
  `modified` exists (evidence has none, so it always merges).

## Out of scope

- Raw SIEM, streaming, or event-volume scale.
- A full SIEM event model (hosts/users/processes) — v1 is flattened entities
  (a multi-field event becomes multiple rows).
- Learned/probabilistic similarity (Jupyter).

## Delivery stages

1. Evidence import command (accumulating `observable` nodes).
2. `coverage --from-graph` (score the accumulated set).
3. A Jupyter producer sample + docs.

## Risks

- **Identity collisions** — canonicalization decides what merges; domain/hash
  are safe, file-names are case-sensitive (kept as-is).
- **Order across windows** — logical `order` must be produced consistently, or
  the ordered view is noise.
- **Evidence/graph co-residency** — mixing evidence with the CTI reference in
  one graph needs `source_feed` discipline (shared with the reference is a
  feature, not a bug, for matching).

## Open decisions (resolved for v1)

- **Evidence node label:** reuse `observable` (recommended) — the same layer
  the DFIR workspace already produces, so evidence and CTI observables
  co-reside naturally.
- **Occurrence model:** merged node only (v1) — one node per identity; each
  observation is recorded in the node's history. Per-observation nodes are a
  later option if occurrence-level queries are needed.
- **Graph:** the same graph as the CTI reference (distinguished by
  `source_feed`) — co-residency is a feature for matching.
- **`--from-graph` selection:** optional `--source <window>`; default = all
  accumulated evidence. Order/range selection deferred.
