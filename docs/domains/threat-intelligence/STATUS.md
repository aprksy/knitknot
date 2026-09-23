# Threat Intelligence — Current State

> **Living document.** A point-in-time snapshot of what users can rely on
> from the Threat Intelligence extension in this build. Updated as the
> extension changes. Last updated: 2026-09-22.
>
> Design intent lives in [README.md](README.md). This file describes
> behavior as it is. If a statement here disagrees with reality, reality
> wins and this file is wrong — file an issue or send a PR.

Stage 1 is implemented and shipped: STIX 2.1 bundle import into the graph,
queryable through the existing DSL, REPL, and exporters.

What's new since Stage 1: the graph kernel now preserves append-only
version history across re-imports (ADR 0002) — see "Version history" below.

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
  `object-ref` all work in `Has(...)`; CTI verbs persist through Save/Load
  like any other verbs.
- **Downstream tooling** — imported graphs query, export to DOT/SVG/JSON,
  and save to `.gob` like any other graph.

---

## What users should not expect

### Version history

Version history is now preserved across re-imports: re-importing an
updated object appends a new snapshot instead of destroying the old
claim (graph kernel, ADR 0002). Temporal queries (e.g. "what did we
know about this indicator on date X") are not yet surfaced in the DSL
but the underlying primitives exist (`GetNodeAt`, `GetNodeHistory`,
event-log queries on `VersionedStorage`).

### No STIX export

`export --format json` gives the generic `{nodes, edges, verbs}` document,
not a STIX bundle. Round-trip today means import-idempotency, not STIX-out.

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

`Has()` is outgoing-only with a required match value; no bounded multi-hop,
no "follow `uses` with no predicate." CTI-shaped questions needing raw
traversal wait on the graph-layer work.

### Always pass `-f`

Without it the import runs, prints success, and evaporates with the
process. Easiest way for a new user to lose work.

### Analyst-sized only

No ingestion benchmarks; a few thousand objects is comfortable territory,
tens of thousands is uncharted. No TAXII, no polling, no enrichment — feed
plumbing is the operator's job.

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

Incoming traversal, unique STIX-ID constraints, edge/property indexes,
durable storage backend, STIX export, marking enforcement, TAXII,
point-in-time queries. None block the current importer.

---

## How to update this file

When the extension gains a capability, fix a limitation, or changes a
behavior: update the relevant section here in the same PR. Keep the
blueprint ([README.md](README.md)) as the design record and this file as
the behavior record; don't let them contradict each other.
