# Domain Implementation Template

Copy this file to `docs/domains/<domain-name>/README.md` and replace each
section. Delete sections that do not apply; do not leave `TODO` placeholders.

> **Status:** proposed | experimental | supported
>
> **Owner:** team or maintainer
>
> **Last reviewed:** YYYY-MM-DD

## Problem statement

Describe the user problem and why a graph is a suitable representation.
Include the expected graph size and deployment model.

## Scope

### In scope

- Concrete capability

### Out of scope

- Concrete non-goal

## Domain model

### Node types

| Type | Identity | Required properties | Notes |
| --- | --- | --- | --- |
| Example | Stable key | `name` | Domain meaning |

### Edge types

| Kind | From | To | Required properties | Direction |
| --- | --- | --- | --- | --- |
| relates-to | Example | Example | None | Outgoing |

### Property conventions

Document names, types, units, normalization, optionality, and null handling.

## Identity and deduplication

Define stable IDs, canonicalization, merge behavior, aliases, and conflict
resolution. State whether updates overwrite, append versions, or create new
nodes.

## Temporal model

Define created/modified/validity timestamps, history retention, revocation,
and how point-in-time queries are represented.

## Provenance

Define source attribution, confidence, evidence, lineage, and how conflicting
claims coexist.

## Validation

List domain invariants and identify the trust boundary where validation runs.
Do not move domain rules into the graph kernel.

## Import and export

| Format | Direction | Version | Lossless | Notes |
| --- | --- | --- | --- | --- |
| Example | Import/export | 1.0 | Yes/No | Mapping details |

Document behavior for unknown/custom fields and partial failures.

## Extension registration

List the capabilities this domain intends to register:

- Verbs
- Importers
- Exporters
- Optional commands

Do not add a registry method solely for this domain unless a second domain can
plausibly use the same generic capability.

## Required graph capabilities

List missing foundational capabilities with concrete examples. Keep them
separate from domain behavior.

## Security and access control

Document markings, tenant isolation, authorization, audit, and sensitive-data
handling.

## Testing strategy

- Domain-model unit tests
- Import/export fixture round trips
- Storage-independent contract tests
- Invalid input and boundary tests
- Performance fixture and expected scale

## Delivery stages

1. Smallest useful vertical slice
2. Required correctness work
3. Performance and durable-storage work
4. Optional integrations

## Definition of done

State observable acceptance criteria. Avoid vague goals such as "production
ready" without measurable requirements.

## Open decisions

Record unresolved questions with an owner or decision date.
