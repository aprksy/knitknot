# KnitKnot Architecture

This document explains the design principles and component structure of KnitKnot.

## Design Goals
- **Modular**: Components can be replaced or extended
- **Embeddable**: Easy to use as a Go library
- **Extensible**: Support custom verbs, storage backends
- **Clean Separation**: No circular dependencies

## Component Overview

TODO: *draw component interaction diagram*

## Key Components
TODO: *need edit*
| Component | Description |
| --- | --- |
| GraphEngine | Top-level orchestrator; combines storage, query, and context |
| StorageEngine | Abstraction for node/edge persistence |
| QueryEngine | Parses and executes DSL queries |
| VerbRegistry | Maps relationship types (e.g., has_skill) to semantics |
| Builder | Fluent DSL implementation |
| ResultSet | Immutable result carrier |

## Data Flow 
1. User writes: `Find('User').Has('has_skill', 'Go')`
2. Parser builds `QueryPlan`
3. `QueryEngine` traverses graph using storage
4. Results returned via `ResultSet`
5. Output as `text`, `JSON`, or `DOT`
     

## Future Extensibility 
- Add RocksDBStorage backend
- Support HTTP server
- Enable distributed mode
     