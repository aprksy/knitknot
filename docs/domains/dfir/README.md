# DFIR — Case × CTI Correlation

> **Status:** experimental
>
> **Owner:** knitknot maintainers
>
> **Last reviewed:** 2026-10-08

## Problem statement

A DFIR investigation produces **case observables** — file hashes, domains,
IPs, URLs, file names collected from the environment. A curated CTI graph
holds what is *known* about adversary infrastructure. The recurring
question is the join:

> Which of our case findings are known adversary infrastructure, and who
> is behind them?

A graph is the right representation because the answer is a traversal, not
a lookup: an observable sits on an indicator, which indicates malware,
which a campaign uses, which is attributed to an actor.

**Deployment model:** CLI, one case at a time. The case side is hundreds to
low thousands of observables; the CTI side is whatever was imported. This
is a correlation workload, not an ingestion-scale one.

## Scope

### In scope

- Extract observables from STIX 2.1 indicator patterns
- Correlate a case observable list against a CTI graph (read-side)
- Project a persistable **case workspace graph** — observables + matched
  indicators + reachable context
- Report as text/JSON; the workspace is a normal graph (queryable,
  exportable to Parquet/STIX)

### Out of scope

- Log ingestion, parsing, timelines, time-range queries
- Modeling hosts/users/processes/artifacts (the full DFIR domain model)
- Behavior → ATT&CK technique mapping (needs semantic inference)
- Event-volume scale

## Domain model

### Node types

| Type | Identity | Required properties | Notes |
| --- | --- | --- | --- |
| `observable` | `type` + canonical `value` | `type`, `value`, `name` | Materialized by `BuildWorkspace`; `name` = value so `Has('based-on', <value>)` resolves |
| copied CTI nodes | source graph ID | as carried | `indicator`, `malware`, `attack-pattern`, `campaign`, `intrusion-set` copied verbatim into the workspace |

### Edge types

| Kind | From | To | Required properties | Direction |
| --- | --- | --- | --- | --- |
| `based-on` | indicator | observable | `source_feed` | Outgoing (materialized) |
| `indicates` | indicator | malware | — | Copied from source |
| `uses` | malware | attack-pattern | — | Copied |
| `uses` | campaign | malware | — | Copied (incoming to malware) |
| `attributed-to` | campaign | intrusion-set | — | Copied |

### Property conventions

- `type` — canonical observable token: `domain`, `ipv4`, `ipv6`, `url`,
  `file-name`, `file-hash-sha256`, `file-hash-sha1`, `file-hash-md5`.
- `value` — canonical form (see identity rules); `name` mirrors it.
- `source_feed` — case feed id (`--source`, default `case`) for observables.

## Identity and deduplication

`Observable.Key()` = `type` + NUL + `Canonical()`. Canonicalization:

- `domain` → lowercase, one trailing `.` stripped
- `file-hash-*` → lowercase, whitespace stripped
- `ipv4` / `ipv6` → whitespace trimmed only
- `url` / `file-name` → whitespace trimmed only (paths and names can be
  case-sensitive)

`BuildWorkspace` dedups observable nodes by `Key()` across the whole build,
so a CTI-derived observable and an identical case observable are one node.
Updates to copied CTI nodes are snapshots, not overwrites (the graph
kernel's append-only history).

## Temporal model

**None.** This domain has no time dimension: it does not model event time,
time-range queries, or point-in-time views. Case context is carried as an
optional free-text `context` string, not a timestamp. Anything
timeline-shaped belongs in the DFIR tool that produced the observables, or
in Jupyter.

## Provenance

Observable provenance is `source_feed` (`--source`). Copied CTI nodes keep
their existing provenance properties. There is no confidence propagation,
no evidence chain, and no conflict resolution across feeds — last write
wins, with history.

## Validation

- Case CSV rows: unknown `type` alias, missing `type`, or missing `value`
  fail with the offending row number. Blank lines are skipped.
- Indicator patterns: unsupported constructs are skipped, never matched.
  A case observable with no match yields no entry (never a false hit).
- Validation lives in this domain (`ParseCaseCSV`, `ExtractObservables`),
  not in the graph kernel.

## Import and export

| Format | Direction | Lossless | Notes |
| --- | --- | --- | --- |
| Case CSV (`type,value[,context]`) | Import | Yes | Type aliases accepted; see GUIDE |
| `.gob` workspace | Export | Yes | Written by `correlate --workspace` |
| Parquet (inherited) | Export | Yes | `export --format parquet -f workspace.gob` |
| STIX 2.1 (inherited) | Export | Yes | Workspace CTI nodes carry `stix_id` |

## Extension registration

**None.** This domain does not register verbs, importers, or exporters with
the extension registry. It is a standalone package plus one CLI command
(`knitknot correlate`) that reads a graph through the `StorageEngine` port.
Per `docs/domains/README.md`, it does not import another domain extension:
the STIX vocabulary it needs is declared locally (`extensions/dfir/vocab.go`),
and the test suite imports `extensions/cti` to seed fixtures and guard
against vocabulary drift.

## Required graph capabilities

Missing foundational capabilities this domain would use, with examples:

- **Time comparisons in `Where`** — `Where('n.ts', '>', '2024-…')` does not
  work today (`>`/`<` coerce numbers only). Not on the critical path here,
  but it blocks any timeline-shaped extension.
- **Multi-hop traversal** — the traversal is done in Go, not the DSL. A
  variable-length path would let the workspace questions be expressed
  directly in a query.
- **Cross-graph query** — correlation currently requires the reference and
  case views to be in one storage engine. A workspace projection avoids
  this, at the cost of copying.

## Security and access control

None implemented. Markings on copied CTI nodes ride along as properties and
are **not enforced**. Case observables may be sensitive; the workspace file
inherits the filesystem's protections and nothing more. Do not present this
as TLP-handling or evidence-grade chain-of-custody.

## Testing strategy

- Extractor unit tests (supported clauses, quoting/whitespace, boolean and
  qualifier tolerance, unsupported/malformed input)
- CSV parsing tests (aliases, optional context, error cases)
- Correlation tests over an inline STIX fixture (chain populated, miss
  yields none, canonical matching)
- Workspace tests (projection counts/labels/edges, thin projection, source
  graph unmutated, `based-on` traversal, dedup)
- CLI E2E against `docs/cti/samples/th-sample.json`

## Delivery stages

1. **Shipped:** extractor, read-side correlation, CLI report. *(committed)*
2. **Shipped:** workspace projection, `--workspace`. *(committed)*
3. Optional: `bolt:` workspace output; full observable-layer materialization
   (not only matched); multi-case separation within one workspace.
4. Optional: cross-graph correlation without projection.

## Definition of done

- A case CSV of observables correlates against a CTI graph and reports the
  reachable adversary context, with zero false hits.
- `--workspace` produces a graph that reloads, queries (`Find`,
  `Has('based-on', …)`), and exports to Parquet.
- The source graph is unchanged by a workspace build.
- `go test ./extensions/dfir/...` is green.

## Open decisions

- Whether the observable layer should be materialized for **all** indicators
  (browsable, cluster-analysis input) or only matched ones (current).
- Whether case workspaces should support `bolt:` output like the main store.
- Whether multiple cases in one workspace need first-class separation beyond
  `source_feed`.
