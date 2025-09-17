[![Go Coverage](https://img.shields.io/badge/coverage-86%25-brightgreen)](coverage.html)

# KnitKnot

> *A lightweight, embeddable property graph engine with a fluent query DSL.*

KnitKnot lets you model and explore relationships using an intuitive CLI and REPL. Think of it as **SQLite for property graphs** — simple, fast, and embeddable.

```bash
knitknot repl
knitknot> Find('User').Has('has_skill', 'Go').Where('n.age', '>', 30)
```

## Ideal for: 

- Organizational graphs
- Skill/knowledge mapping
- Event relationship analysis
- Embedded analytics
     
[Full Documentation](https://knitknot.aprksy.dev/docs)
## Features 

- Fluent DSL: `Find(...)`, `Has(...)`, `Where(...)`
- REPL Shell: Interactive exploration with `EXPLAIN`, `DEFINE VERB`
- Persistence: Save/load to .gob binary format
- Export: Generate DOT for Graphviz visualization
- Custom Verbs: Define your own relationship semantics (*teaches*, *owns*, etc.)
- Subgraph Scoping: Query within domains (*org*, *skills*)
- CLI & API: Use as tool or embed in Go apps
     

## Quick Start 
```bash
# Install
go install github.com/aprksy/knitknot@latest

# Start REPL
knitknot repl

# Run query
knitknot query "Find('User')" --file org.gob

# Export to DOT
knitknot export --format dot > graph.dot
dot -Tsvg graph.dot > graph.svg
```
 
## Learn More 
- [DSL Syntax Guide](docs/dsl.md)
- [Architecture](docs/architecture.md)
- [Contribute](docs/CONTRIBUTING.md)

## Changelog
- [Changelog](docs/CHANGELOG.md)
     

## License 

[MIT](docs/LICENSE.md)  © 2025 [aprksy](https://github.com/aprksy)