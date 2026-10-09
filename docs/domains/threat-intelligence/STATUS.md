# Threat Intelligence — Current State

> **Living document.** A point-in-time snapshot of what users can rely on
> from the Threat Intelligence extension in this build. Updated as the
> extension changes. Last updated: 2026-10-08.
>
> Design intent lives in [README.md](README.md). This file describes
> behavior as it is. If a statement here disagrees with reality, reality
> wins and this file is wrong — file an issue or send a PR.

Stage 1 is implemented and shipped: STIX 2.1 bundle import into the graph,
queryable through the existing DSL, REPL, and exporters.

What's new since Stage 1: the graph kernel now preserves append-only
version history across re-imports (ADR 0002) — see "Version history" below.

`[updated 2026-10-08]` Since the last revision the CLI grew an enterprise
path: a durable BoltDB backend with batched imports (ADR 0003), Parquet and
query-scoped exports for the Jupyter handoff, a stale-re-import guard, and
measured import/query performance. Open work is tracked in
"Continuity — open items" at the end of this file.

---

## What works

```bash
knitknot import --format stix-2.1 -f ti.gob bundle.json
knitknot query "Find('malware')" -f ti.gob
knitknot query "Find('indicator').Has('indicates', 'FrostbiteRAT')" -f ti.gob
```

These behaviors are covered by `extensions/cti/*_test.go` and the
`testdata/minimal-bundle.json` fixture:

- **STIX 2.1 bundle import** — SDOs, SCOs (as generic nodes), relationships,
  sightings, markings, language content, and custom objects (`x-*` preserved
  with all raw properties, plus unknown top-level properties on known types).
- **Order-independent** — relationships listed before their referenced
  objects resolve (two-pass: stage + validate, then upsert nodes, then
  create edges).
- **Idempotent re-import** — importing the same bundle twice changes nothing
  (stable STIX-ID lookup + property merge). Safe to re-run a feed pull.
- **Fail-before-mutate** — a relationship pointing at a nonexistent object,
  or an SDO missing `created`/`modified`, errors out with zero partial
  writes.
- **Provenance on every node and edge** — `stix_id`, `type`, `imported_at`,
  plus whatever the object carried (`created_by_ref`, `confidence`,
  `labels`, `external_references`, markings).
- **Revoked objects kept** — `revoked=true` nodes stay addressable; nothing
  is auto-deleted.
- **Registered vocabulary** — `uses`, `targets`, `indicates`,
  `attributed-to`, `communicates-with`, `based-on`, `derived-from`,
  `object-ref`, `has-tactic` all work in `Has(...)`; CTI verbs persist
  through Save/Load like any other verbs.
- **Tactic materialization** `[updated 2026-10-08]` — `import --tactics`
  turns ATT&CK `kill_chain_phases` (otherwise an inert array prop) into
  `x-mitre-tactic` nodes carrying a `rank` and one
  `attack-pattern -has-tactic-> tactic` edge per phase (ADR 0005). Opt-in;
  the default import is unchanged. Rank comes from
  `x-mitre-matrix.tactic_refs` when present, else a fixed table. It is
  **tactic-level (coarse)**: many techniques share a rank, and a
  multi-tactic pattern gets multiple edges (not collapsed).
- **Downstream tooling** — imported graphs query, export to DOT/SVG/JSON,
  and save to `.gob` like any other graph.
- **Durable BoltDB backend** `[updated 2026-10-08]` — `--store bolt:<file.db>`
  stores the graph in a single-file B+tree DB with indexes, so queries no
  longer decode the whole graph (ADR 0003). `sync gob:<in> bolt:<out>`
  migrates an existing `.gob`; batched writes collapse imports into one
  transaction.
- **Parquet + query-scoped export** `[updated 2026-10-08]` —
  `export --format parquet -o <prefix>` writes `<prefix>_nodes.parquet` +
  `<prefix>_edges.parquet` for Jupyter/pandas; `export --format stix
  --query "<dsl>"` emits only the matching subgraph as a focused STIX
  bundle (the "hypothesis → focused bundle" workflow).
- **Stale re-import guard** `[updated 2026-10-08]` — a re-import strictly
  older than the stored snapshot (by STIX `modified`) is skipped: live
  props stay current, no snapshot is appended, and the count is reported on
  stderr. Equal or unparseable timestamps keep the old merge behavior.
- **Multi-target verbs** `[updated 2026-10-08]` — `uses` and
  `attributed-to` match across node labels (malware, attack-patterns,
  intrusion-sets, …), so `Find('malware').Has('uses', '<TTP name>')` and
  `Find('campaign').Has('attributed-to', '<actor>')` work.
- **Measured performance** `[updated 2026-10-08]` — see "Analyst-sized
  only" for the numbers.

---

## What users should not expect

### Version history

The graph kernel preserves append-only version history across re-imports
(ADR 0002; nodes in `0893741`, edges in `51b8e89`, tombstone cascade on
node delete in `05ec3c7`): every change appends a
`Snapshot{Source, LogicalTime, Transaction}`, and re-importing an updated
STIX SDO keeps v1 and v2 instead of overwriting. Delete is a tombstone,
not real removal — `GetNode`/`GetEdge` still return tombstones for
history access. Kernel primitives exist: `GetNodeAt(id, t)` /
`GetNodeHistory(id)` / `GetEvents(revFrom, revTo)` /
`GetEventsByTransaction(txID)`. Not yet exposed: a temporal DSL surface
(`.AsOf('2024-04-01')`) — "what did the graph look like on date X" is
only queryable via the kernel primitives directly, not through the
parser yet.

`[updated 2026-10-08]` Re-imports are now ordered by STIX `modified`: a
bundle strictly older than the stored snapshot is skipped (live props and
history unchanged, count on stderr). Equal timestamps still merge, and an
identical re-import still appends a snapshot — there is no content no-op
detection.

### STIX export

`knitknot export --format stix` works (Stage 2 item 1, `ce87c1d`):
stix_id-gated export (only nodes carrying a `stix_id` are emitted),
`CustomObject`-based lossless round-trip, deterministic sort by STIX ID,
and a trailing `# knitknot skipped N nodes, M edges` summary line only
when something was skipped. Not supported: the typed-constructor path
and strict schema validation.

### No pattern parsing or schema validation

An indicator's `pattern` is stored as a string; nothing compiles or matches
it. Malformed-but-parseable STIX passes through (unknown fields preserved,
invalid structure rejected only where the library's own rules trip).

### Markings stored, not enforced

TLP and granular markings ride along as properties. Nothing filters on
them. Do not present this to a compliance officer as TLP handling.

### `source_feed` is set at import time

`knitknot import --source <feed-id>` populates `source_feed` on every
imported node and edge, and the value flows into the versioning
`Snapshot.Source` / event-log `Source` so provenance is queryable. A
feed id is still only as good as the operator's discipline — there is no
feed registry or validation of the id itself — but the structural
attribution is populated.

### Single-writer upsert, no transactions

Lookup-before-add races under concurrent writers; a storage error mid-pass-2
(disk full) can leave a half-imported bundle. One import at a time — same
rule as the core lib.

### Querying is the core DSL's querying

`Has()` is outgoing-only, requires a match value, and fans out from the
`Find` node. `[updated 2026-10-08]` **Multi-hop now works** (ADR 0004):
`.Follow(rel, dir)` / `.FollowHas(rel, value, dir)` chain fixed-length hops
from the previous node, and `.Reach(rel, dir, maxDepth)` /
`.ReachHas(rel, value, dir, maxDepth)` do bounded reachability (1..maxDepth
hops, default 8, capped 32, cycle-safe) — so
`Find('intrusion-set').Follow('attributed-to','in').Follow('uses','out')` and
"is X reachable within N steps" (`ReachHas`) are both one query. Direction is
`'out'`/`'in'`/`'both'`. Still missing: path enumeration / constrained paths
("via node X") and shortest path (ADR 0004 Tier 3). `Where()` supports only
`=`, `!=`, `>`, `<` — there is no `contains`/regex, and `>`/`<` coerce
numbers only, so **date-range filters do not work**
(`Where('n.valid_from', '>', '2024-…')` matches nothing; verified). Exact
timestamps with `=` work; for ranges, export and filter downstream.

`[updated 2026-10-08]` **Custom fields are preserved but only scalar ones
filter.** MITRE `x_mitre_*` fields and any custom properties survive import,
STIX round-trip, Parquet (`props_json`), and copy into a DFIR workspace
verbatim. But `Where` filters **scalar** values only: `Where('n.x_mitre_version',
'=', '1.0')` works, while **array and boolean fields do not**
(`Where('n.x_mitre_domains', '=', 'enterprise-attack')` and
`Where('n.x_mitre_deprecated', '=', 'false')` both match nothing; verified).
For those, export and filter downstream.

### Always pass `-f`

Without it the import runs, prints success, and evaporates with the
process. Easiest way for a new user to lose work.

### Analyst-sized only `[updated 2026-10-08]`

Benchmarked on a 4,000-node / 20,565-edge proxy slice of MITRE ATT&CK
(`gob` / `bolt`, post-fix):

| objects | gob import | bolt import | gob query | bolt query |
| --- | --- | --- | --- | --- |
| 1,508 | 0.17 s | 0.55 s | ~0.02 s | ~0.01 s |
| 24,565 | 7.7 s | 22.9 s | ~4–5 s | 0.014–0.15 s |

Bolt trades slower imports for order-of-magnitude faster queries — the
right trade for import-once/query-many, which is why it is the recommended
backend past a few thousand objects. **Not yet verified:** the full
26,086-object / 52 MB `enterprise-attack.json` never completed on the WSL
dev host (host crashes, not importer errors); the curve above projects it
in tens of seconds, but that number is unproven until it runs on a real
Linux host. No TAXII, no polling, no enrichment — feed plumbing is the
operator's job.

---

## Extension surface (Stage 1)

| Piece | Location | Notes |
| --- | --- | --- |
| Contract + registry | `extensions/extension.go` | `Extension`, `Importer`, `Exporter`, `Registry` + `NewRegistry()`; duplicate/empty/nil/post-freeze writes rejected |
| Import command | `cmd/import.go` | `knitknot import --format <name> -f <file.gob> <input>`; `registerExtensions` hook wires `cti.New()` |
| Vocabulary | `extensions/cti/vocabulary.go` | 11 node labels, 8 edge kinds, STIX + provenance property names |
| Identity | `extensions/cti/identity.go` | STIX ID parse; domain/hash/CVE/URL canonicalizers |
| Importer | `extensions/cti/stix_import.go` | Two-pass, via `TcM1911/stix2 v0.10.4` |
| Extension entry | `extensions/cti/extension.go` | `Name()="threat-intelligence"`; registers `stix-2.1` + verbs |
| Fixture | `extensions/cti/testdata/minimal-bundle.json` | 6 nodes + 3 relationships; rel listed before its refs |

Dependency added for Stage 1: `github.com/TcM1911/stix2 v0.10.4`
(`google/uuid` indirect). Alternatives ruled out in research:
`freetaxii/libstix2` (write-oriented), `oasis-open/cti-go-stix`
(broken module path), `qensus-labs/go-stix` (too young, heavy deps).

---

## Deferred to Stage 2+

`[updated 2026-10-08]` Shipped since this list was written: durable storage
backend (BoltDB, ADR 0003), edge/property indexes (inmem + bolt), STIX
export, and Parquet/query-scoped export.

Still open: incoming traversal, unique STIX-ID constraints enforced at the
store layer, marking enforcement, TAXII, point-in-time queries (`.AsOf`),
and the DSL date/`contains` operators noted above. None block the current
importer.

---

## Continuity — open items `[added 2026-10-08]`

Ordered by what to pick up first. Everything above is shipped and tested;
this is verification and hardening, not feature work.

1. **Run the full `enterprise-attack.json` import on a real Linux host.**
   Never completed on WSL (two host crashes at ~400 MB+ RSS). Proxy slice
   imports fine; the full-file number is the last unproven performance
   claim. Compare `gob` vs `bolt` and record both.
2. **Add CI** (GitHub Actions): `go test ./...` on push. Tests are green
   locally but nothing guards against regressions.
3. **Exercise Parquet + `--query` export on the bolt backend.** Verified on
   `gob` only; the code paths are backend-agnostic but untested there.
4. **Consider `import --strict`** to escalate stale-skip from an stderr
   line to a non-zero exit, for feed automation that wants to fail loudly.
5. **Consider no-op re-import detection** (skip appending a snapshot when
   content is byte-identical) to stop history growing on repeated pulls.
6. **Cross-backend re-sync assumes a shared ID space** (flagged in the bolt
   work). One-shot `gob → bolt` migration is exact; repeated re-sync across
   divergent ID spaces can false-match. Document or guard before relying on
   it.
7. **Add an enterprise-scale demo** — the shipped demo covers a 25-object
   fixture; a pipeline demo (import → DSL → Parquet → networkx) would show
   the intended Jupyter handoff end to end.

Pre-existing Stage-2 items (marking enforcement, TAXII, `.AsOf`, DSL
date/`contains` ops) are listed above and unchanged.

---

## How to update this file

When the extension gains a capability, fix a limitation, or changes a
behavior: update the relevant section here in the same PR. Keep the
blueprint ([README.md](README.md)) as the design record and this file as
the behavior record; don't let them contradict each other.
