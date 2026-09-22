# Domain Implementations

> **Status: design guidance.** KnitKnot does not yet expose the extension
> interfaces described here. This directory defines the boundary that future
> implementations should follow before code is added.

KnitKnot's graph kernel must stay business-domain neutral. Threat
intelligence, fraud, supply chain, identity, healthcare, and future domains
belong in separate extensions that depend on the kernel; the kernel must not
depend on them.

## Directory convention

Each business domain gets its own documentation directory:

```text
docs/domains/
├── README.md
├── TEMPLATE.md
└── <domain-name>/
    └── README.md
```

Current domains:

- [Threat Intelligence](threat-intelligence/README.md)

When implementation begins, the candidate source layout is:

```text
extensions/
├── extension.go          # minimal extension and registry contracts
└── <domain-name>/        # domain-owned vocabulary and adapters
```

The source layout is a proposal, not a commitment. Do not create empty
packages before a domain implementation exists.

## Three boundaries

### 1. Foundational graph domain

The graph kernel owns only generic graph concepts and invariants:

- Nodes, edges, properties, IDs, and subgraph membership
- Node and edge CRUD
- Traversal and generic query plans
- Generic filtering and limits
- Storage and query ports
- Invariants such as unique IDs, valid edge endpoints, and cascading edge
  deletion

It must not know what a threat actor, customer, shipment, patient, CVE, or
STIX object is.

### 2. Implementation adapters

Adapters implement foundational ports or expose the kernel to users:

- In-memory storage, indexes, locking, and persistence
- Future durable storage backends
- Gob persistence
- DOT, SVG, and JSON exporters
- CLI and REPL integration
- Concrete query execution

Adapters may depend on the graph kernel. The graph kernel must not import a
concrete adapter.

### 3. Business-domain extensions

A domain extension owns business meaning:

- Vocabulary and relationship semantics
- Domain validation
- Import/export mappings
- Normalization and identity rules
- Provenance conventions
- Domain-specific commands or workflows

Extensions depend on graph ports and public graph types. They must not import
`cmd` or a concrete storage package such as `pkg/storage/inmem`.

## Dependency rule

Dependencies point inward:

```text
cmd / application composition
        /             \
infrastructure      domain extensions
        \             /
           graph kernel
```

Forbidden dependencies:

- Graph kernel → business domain
- Graph kernel → CLI
- Graph kernel → concrete storage
- Domain extension → CLI or concrete storage
- One domain extension → another domain extension, unless an explicit shared
  domain contract is introduced

## Candidate extension mechanism

Start with compiled-in extensions. Do not build a runtime plugin loader until
there is a demonstrated deployment need.

A minimal candidate contract is:

```go
type Extension interface {
    Name() string
    Register(Registry) error
}

type Registry interface {
    RegisterVerb(name string, verb types.Verb) error
    RegisterImporter(format string, importer Importer) error
    RegisterExporter(format string, exporter Exporter) error
}
```

This interface is intentionally not implemented yet. The first real domain
extension should validate the design before it enters the public API.

Registration rules:

1. Register extensions during application startup.
2. Freeze the registry before serving commands or queries.
3. Reject duplicate extension names and format names.
4. Abort startup if registration fails; do not leave partial registration.
5. Avoid package `init()` registration. Composition must be explicit.
6. Do not give extensions unrestricted application or storage internals.
7. Commands discover registered capabilities; extensions do not import Cobra.

## Extension versus plugin

An **extension contract** defines how an optional capability integrates. A
**plugin loader** defines how code is delivered at runtime. They are separate
decisions.

Use compiled-in extensions first:

```go
app := NewApp(
    WithStorage(inmem.New()),
    WithExtension(threatintelligence.New()),
)
```

Avoid Go's `plugin` package as the default mechanism. It is platform-specific,
requires matching compiler and dependency versions, and gives arbitrary code
access to the host process. If runtime third-party loading becomes necessary,
prefer a versioned subprocess, gRPC, or WASM contract.

## Admission checklist for a new domain

A domain implementation is ready to add when its document answers:

- What user problem does this domain solve?
- Which concepts are nodes, edges, and properties?
- What identity and deduplication rules apply?
- What provenance and temporal rules apply?
- Which imports and exports are required?
- Which validations belong to the domain rather than the graph kernel?
- Which graph capabilities are missing, with concrete use cases?
- How is the extension tested without a concrete storage dependency?
- Which security or access-control rules must be enforced?
- What is explicitly out of scope?

Use [TEMPLATE.md](TEMPLATE.md) for new domains.

## Design principle

> A business domain may prove a new extension capability, but it must not
> reshape the foundational graph model around its own vocabulary.
