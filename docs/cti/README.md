# CTI Analysis with KnitKnot

KnitKnot is a lightweight, embeddable graph engine for **cyber threat intelligence (CTI)**. It ingests STIX 2.1 bundles, normalizes them into a property graph, and lets you query, analyze, and export the results.

## What it does

```
STIX bundle → knitknot graph → DSL queries → Parquet/STIX export → Jupyter analysis
```

- **Ingest** published threat intelligence (MITRE ATT&CK, vendor reports, OSINT feeds)
- **Query** the graph with a fluent DSL: `Find('campaign').Has('uses', 'malware-x')`
- **Export** as Parquet (for Jupyter/pandas), STIX (for sharing), DOT (for visualization)
- **Bridge** to Jupyter for statistical analysis, clustering, and detection engineering

## What it is not

- Not a SIEM — operational alert data goes directly to Jupyter, not through KnitKnot
- Not a platform — no server, no web UI, no multi-user. It's a CLI tool and embeddable library
- Not a replacement for OpenCTI — it's the lightweight "laptop" to OpenCTI's "desktop"

## Quick start

```bash
# Import a STIX bundle
knitknot import --format stix-2.1 --source mitre -f graph.gob bundle.json

# Query the graph
knitknot query "Find('campaign').Has('uses', 'frostbiterat')" -f graph.gob

# Export for Jupyter analysis
knitknot export --format parquet -f graph.gob -o analysis/

# Export a focused subgraph as a STIX bundle
knitknot export --format stix --query "Find('campaign').Has('uses', 'frostbiterat')" -f graph.gob -o focused.json
```

## Tutorials

- [Threat Intelligence (TI) tutorial](ti-tutorial.md) — analyze a published attack, understand TTPs, share findings
- [Threat Hunting (TH) tutorial](th-tutorial.md) — form hypotheses, extract patterns, bridge to Jupyter

## Sample files

- [`samples/ti-sample.json`](samples/ti-sample.json) — small STIX bundle for the TI tutorial
- [`samples/th-sample.json`](samples/th-sample.json) — small STIX bundle for the TH tutorial

## Architecture

See [ADR 0001](../adr/0001-pluggable-storage-backends.md) (pluggable backends),
[ADR 0002](../adr/0002-append-only-versioning.md) (versioning),
[ADR 0003](../adr/0003-bolt-backend.md) (BoltDB backend).

## Ecosystem

| Tool | Role |
|---|---|
| **knitknot** | curated CTI graph: ingest, query, export |
| **stix2** | STIX data model + serialization |
| **OpenCTI** | full CTI platform (heavyweight) |
| **networkx** | graph algorithms (clustering, centrality) |
| **Jupyter** | statistical analysis, detection engineering |

Knitknot is the **lightweight graph layer** between raw STIX data and heavy platforms.
