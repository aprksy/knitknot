# Threat Intelligence Domain

> **Status:** proposed design; not implemented
>
> **Owner:** unassigned
>
> **Last reviewed:** 2026-09-22

This document defines how Threat Intelligence (CTI) should integrate with
KnitKnot without coupling the foundational graph kernel to CTI vocabulary.

## Problem statement

CTI analysts need to connect threat actors, campaigns, malware, tools,
vulnerabilities, indicators, infrastructure, identities, reports, and the
claims made about them. A property graph is useful for local exploration and
multi-hop relationship analysis.

The first target is an **embedded, analyst-sized CTI investigation graph**.
It is not an authoritative multi-tenant threat-intelligence platform.

## Scope

### In scope for the first extension

- STIX 2.1 bundle import
- Mapping common STIX objects and relationships to graph nodes and edges
- Stable STIX ID upsert
- Basic observable normalization
- Provenance, confidence, and temporal properties
- STIX 2.1 export for imported objects that have not lost information
- Local querying through the existing graph DSL

### Out of scope initially

- TAXII client/server
- Automated enrichment or external feed polling
- Ontology reasoning, RDF, OWL, or SPARQL
- Multi-tenant authorization
- Distributed ingestion or sharding
- Automatic entity resolution across weak or conflicting identities
- Enforcement of data-sharing agreements beyond stored markings

## Boundary

The CTI extension may import:

- Public graph types
- Storage/query ports
- Generic extension registry contracts

It must not import:

- `cmd`
- `pkg/storage/inmem`
- Gob persistence internals
- DOT/SVG implementation details

The foundational graph packages must never import this extension.

Candidate implementation layout:

```text
extensions/threatintelligence/
├── extension.go       # registration only
├── vocabulary.go      # labels, edge kinds, property names
├── identity.go        # STIX IDs and observable canonicalization
├── validate.go        # CTI-domain invariants
├── stix_import.go     # STIX 2.1 -> graph mapping
├── stix_export.go     # graph -> STIX 2.1 mapping
└── testdata/          # small, licensed STIX fixtures
```

Do not create these packages until the extension contract is accepted.

## Domain model

The graph kernel still sees only generic `Node` and `Edge` values. The CTI
extension owns these conventions.

### Node labels

| Label | STIX concept | Stable identity | Required properties |
| --- | --- | --- | --- |
| `threat-actor` | Threat Actor SDO | STIX ID | `name`, `created`, `modified` |
| `campaign` | Campaign SDO | STIX ID | `name`, `created`, `modified` |
| `malware` | Malware SDO | STIX ID | `name`, `is_family` |
| `tool` | Tool SDO | STIX ID | `name` |
| `vulnerability` | Vulnerability SDO | STIX ID | `name` (normally CVE) |
| `indicator` | Indicator SDO | STIX ID | `pattern`, `pattern_type`, `valid_from` |
| `infrastructure` | Infrastructure SDO | STIX ID | `name` |
| `identity` | Identity SDO | STIX ID | `name`, `identity_class` |
| `report` | Report SDO | STIX ID | `name`, `published` |
| `observed-data` | Observed Data SDO | STIX ID | `first_observed`, `last_observed` |
| `observable` | STIX SCO | STIX ID or canonical key | type-specific value |

Unknown STIX object types should be preserved as generic nodes with their
original `type` and raw custom properties, not discarded.

### Edge kinds

STIX SRO `relationship_type` values map directly to opaque edge kinds, for
example:

| Kind | Typical from | Typical to | Direction |
| --- | --- | --- | --- |
| `uses` | threat-actor/campaign | malware/tool/infrastructure | Outgoing |
| `targets` | threat-actor/campaign/malware | identity/vulnerability | Outgoing |
| `indicates` | indicator | malware/campaign/threat-actor | Outgoing |
| `attributed-to` | campaign/intrusion-set | threat-actor | Outgoing |
| `communicates-with` | observable/infrastructure | observable/infrastructure | Outgoing |
| `based-on` | indicator | observed-data/observable | Outgoing |
| `derived-from` | any STIX object | any STIX object | Outgoing |
| `object-ref` | report/grouping | referenced object | Outgoing |

Direction must follow STIX `source_ref -> target_ref`. KnitKnot currently
traverses outgoing edges only; inverse analysis requires either an incoming
traversal capability in the graph kernel or explicit reverse edges. Do not
silently create reverse edges during import.

### Common properties

Preserve at least:

- `stix_id`, `type`, `spec_version`
- `created`, `modified`
- `created_by_ref`
- `revoked`
- `confidence`
- `lang`
- `external_references`
- `object_marking_refs`, `granular_markings`
- `labels`
- Source/feed identifier and import timestamp

Property names should match STIX names unless a collision with a graph-owned
field requires a documented mapping.

## Identity and upsert

### STIX objects

- Primary key: STIX `id`.
- Version key: `(id, modified)` for SDOs/SROs where `modified` exists.
- Importing the same version must be idempotent.
- A newer version must not destroy the older claim until a retention policy is
  explicitly chosen.
- Revoked objects remain addressable and carry `revoked=true`; do not delete
  their node automatically.

### Cyber-observable objects

Canonicalization must be type-specific:

- Domain names: lowercase, trim trailing dot where appropriate
- IPv4/IPv6: canonical textual form
- URLs: documented URL normalization; do not guess away meaningful parts
- File hashes: lowercase hex, keyed by algorithm + digest
- CVEs: uppercase canonical identifier

Do not merge entities using display names alone. Alias/entity-resolution rules
require explicit provenance and confidence.

The current graph kernel has no unique property index or atomic upsert. Those
are prerequisites for concurrent or large-feed ingestion; the first prototype
may use a single-writer lookup-before-add path with a documented ceiling.

## Temporal model

CTI changes over time. Overwriting the current property map is insufficient
for an authoritative store.

The extension should distinguish:

- Object lifecycle: `created`, `modified`, `revoked`
- Intelligence validity: `valid_from`, `valid_until`
- Observation window: `first_observed`, `last_observed`, `number_observed`
- Relationship activity: `start_time`, `stop_time`
- Import metadata: feed source and ingestion timestamp

For the first slice, preserve these values as properties and keep all imported
versions. Point-in-time query semantics are a later graph/query capability.

## Provenance

Every imported object and relationship must retain:

- Producing organization or feed
- Original STIX ID and version
- Import timestamp
- Confidence (when supplied)
- Object and granular markings
- Raw custom properties not understood by the extension

Conflicting claims coexist. The extension must not collapse two producers'
claims merely because the displayed values match.

## Validation

At the import trust boundary, reject or quarantine:

- Invalid STIX IDs
- Missing required properties
- Relationships whose source or target cannot be resolved after the complete
  bundle is processed
- Invalid timestamps or impossible validity windows
- Unsupported pattern versions when no lossless fallback is available

Unknown custom fields are preserved. Unknown object types are preserved as
generic nodes. Partial bundle failure behavior must be explicit: atomic import
is preferred once the storage layer supports transactions; until then, stage
and validate the bundle before graph mutation.

## Import and export

| Format | Direction | First supported version | Lossless target |
| --- | --- | --- | --- |
| STIX JSON Bundle | Import/export | 2.1 | Yes for supported objects |
| KnitKnot JSON | Export | Current | Graph snapshot, not STIX |
| TAXII | Deferred | — | — |

STIX import should be two-pass:

1. Validate, normalize, and stage all objects.
2. Upsert nodes, then resolve and create relationships.

This avoids relationship-order dependence inside a bundle.

STIX export should fail clearly when graph data cannot be represented
losslessly. It must not fabricate required STIX fields silently.

## Candidate extension registration

The CTI extension would register:

- Importer: `stix-2.1`
- Exporter: `stix-2.1`
- Common CTI relationship verbs such as `uses`, `targets`, `indicates`, and
  `attributed-to`

Normalization and validation remain internal to the importer initially. Do
not add generic `Normalizer`, `Validator`, or `Enricher` registry interfaces
until another domain proves they are shared capabilities.

## Required foundational capabilities

Before calling KnitKnot a production CTI knowledge store, the graph layer
needs:

1. Unique property constraints or an atomic upsert by stable key
2. Edge indexes by source, destination, and kind
3. Property indexes by `(label, property, value)`
4. Incoming traversal and bounded variable-depth traversal
5. Transactions or staged atomic mutation for bundle import
6. A durable storage adapter with crash recovery
7. Pagination and limits applied during traversal, not only after expansion
8. Query support for temporal ranges

These are generic graph capabilities. They should be implemented without CTI
imports or terminology.

## Security and markings

The first prototype may preserve markings without enforcing them, but it must
make that limitation visible. A supported CTI store eventually needs:

- TLP and object marking enforcement on query/export
- Granular marking preservation
- Tenant/feed isolation
- Audit logging
- Authorization decisions before data leaves the process

Do not claim TLP compliance when markings are stored but not enforced.

## Delivery stages

### Stage 1 — embedded CTI prototype

- Minimal compiled-in extension registry
- STIX 2.1 bundle importer
- Stable-ID lookup and single-writer upsert
- Common nodes and relationships
- Provenance/temporal property preservation
- Fixture-based round-trip tests

### Stage 2 — correctness

- Version history and revocation behavior
- Bundle staging and validation
- STIX 2.1 exporter
- Unknown/custom-field preservation tests
- Incoming traversal

### Stage 3 — storage and scale

- Durable storage adapter
- Unique/property/edge indexes
- Transactional import
- Benchmarks using representative CTI bundles

### Stage 4 — controlled sharing

- Marking enforcement
- Tenant/feed isolation
- Audit events
- Optional TAXII integration

## Definition of done for Stage 1

- Importing the same STIX bundle twice is idempotent.
- A relationship may appear before its referenced objects in the bundle.
- Invalid required fields fail before graph mutation.
- Unknown custom fields survive import/export.
- Every node and edge records source and import timestamp.
- At least one fixture covers each supported node and relationship type.
- Tests run against the storage port, not `inmem.Storage` directly.
- The foundational graph packages contain no CTI imports or vocabulary.

## Open decisions

- Whether imported STIX versions become separate graph nodes or version
  records attached to one logical node
- How custom STIX objects map without losing round-trip fidelity
- Whether markings are enforced in the extension, query engine, or an
  application policy layer
- Which durable storage adapter should be the first supported backend
- Whether runtime extension loading is ever needed; compiled-in composition
  is the default until proven otherwise
