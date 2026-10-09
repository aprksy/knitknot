# ADR 0005 — Tactic (kill-chain phase) modeling for ordered coverage

- **Status:** proposed
- **Date:** 2026-10-08
- **Deciders:** knitknot maintainers

## Context

A candidate similarity measure ("ordered coverage") compares the sequence of
an accumulated evidence graph against the sequence implied by a STIX attack
pattern. That needs a **rank/sequence on the STIX side**. MITRE ATT&CK
carries it, but knitknot cannot currently use it:

- **`kill_chain_phases`** on every `attack-pattern` — verified on the
  enterprise bundle: **858/858** patterns carry them, **1090** phases total,
  all `kill_chain_name = mitre-attack`, and **195 patterns are multi-tactic**
  (≥2 phases). It imports as an **array property**, which the DSL cannot
  filter (the array/operator gap), so it is inert.
- **`x-mitre-tactic`** objects (15) carry `x_mitre_shortname` (e.g.
  `credential-access`) matching `phase_name`, plus `name` and
  `external_references[].external_id` (`TA0001`…).
- **`x-mitre-matrix.tactic_refs`** is the **authoritative canonical order** —
  verified: `reconnaissance, resource-development, initial-access, execution,
  persistence, privilege-escalation, stealth, defense-impairment,
  credential-access, discovery, lateral-movement, collection,
  command-and-control, exfiltration, impact`. Note this is **not** numeric TA
  order (TA0042/TA0043 sort last numerically but belong first).
- Our hand-written fixtures carry **no** tactics, so nothing exercises this.

**Granularity caveat (binding):** this yields a **tactic-level partial order**
— many techniques share a rank. It does not give a fine technique sequence;
that exists only in the evidence (Jupyter logical time) and report prose.

## Decision

Materialize tactics as first-class graph structure, **opt-in** (default
import unchanged):

1. **Tactic nodes** — one per phase, identity = `x_mitre_shortname`,
   label `x-mitre-tactic` (verbatim, consistent with import labeling).
   Props: `name`, `external_id`, `rank`. Seeded from imported
   `x-mitre-tactic` objects; **synthesized (minimal)** when a bundle has
   `kill_chain_phases` but no tactic objects.
2. **Rank** — 1-based position in `x-mitre-matrix.tactic_refs` when a matrix
   is present; otherwise a small fixed fallback table (see Open decisions).
3. **New edge kind `has-tactic`** — `attack-pattern → tactic`, one edge per
   phase (multi-tactic is natural, not collapsed). Registered in the CTI
   vocabulary + verb registry.
4. **Optional scalar** `x_mitre_phase` = first phase — a cheap `=`-filterable
   convenience given the array gap. Not a substitute for the edges.
5. **Trigger** — an explicit step (`import --tactics`, or a
   `materialize-tactics` command), so the default import stays lean.

## Semantics

- **Rank** is tactic-level; `stealth` and `credential-access` are adjacent
  ranks, but two techniques in the same tactic are unordered relative to each
  other.
- **Multi-tactic**: no scalar collapse — the edge set is the truth; a
  consumer (ordered coverage) applies its own rule (min rank / all ranks).
- **Idempotent**: stable `x_mitre_shortname` identity means re-running
  materialization merges, not duplicates.
- **Absent data**: bundles without tactics degrade gracefully (no
  nodes/edges, or fallback ranks) — never fabricated.
- **Non-ATT&CK kill chains**: only `kill_chain_name = mitre-attack` is
  ranked; others are noted/skipped.

## Out of scope

- Ordered coverage / the similarity score itself (separate decision).
- Fine-grained technique sequencing (not present in STIX).
- Versioning the tactic table against ATT&CK releases.

## Consequences

- Tactics become queryable and traversable:
  `Find('attack-pattern').Follow('has-tactic', 'out')`,
  `Find('attack-pattern').Has('has-tactic', 'credential-access')`.
- Unblocks the coarse ordered-coverage similarity measure.
- Reuses ADR 0004 multi-hop to walk tactic sequences.
- Adds vocabulary surface (`has-tactic`, rank) the CTI extension must own.

## Delivery stages

1. Vocabulary + `has-tactic` edge kind + tactic node augmentation at import
   (rank from matrix), opt-in flag.
2. Optional `x_mitre_phase` scalar.
3. A tactic-bearing fixture + tests (multi-tactic, no-matrix fallback,
   no-tactics bundle).

## Risks

- **Fallback table drift** — ATT&CK adds/reorders tactics (Stealth TA0005,
  Defense Impairment TA0112 are recent). Prefer the matrix; version the
  fallback.
- **Synthesized nodes on a verbatim label** — a tactic node created without
  an `x-mitre-tactic` object is slightly impure; document it.
- **Over-trusting order** — tactic rank is coarse and real attacks loop
  phases; ordered coverage must be presented as approximate.

## Open decisions (resolved for v1)

- **Node label:** keep the verbatim `x-mitre-tactic` (consistent with import
  labeling); a curated `tactic` label is the alternative if ergonomics argue
  for it later.
- **Trigger:** `import --tactics` flag (opt-in), not a standalone command —
  the data is already read during import.
- **`x_mitre_phase` scalar:** **deferred** — the `has-tactic` edges already
  give queryability; revisit only if direct scalar filtering proves needed.
- **Fallback table:** a small fixed in-code table (the verified canonical
  15), used only when a bundle has no matrix; version it and prefer the
  matrix whenever present.
