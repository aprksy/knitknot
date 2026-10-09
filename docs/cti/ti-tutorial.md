# Threat Intelligence (TI) Tutorial

Analyze a published attack using KnitKnot. This tutorial walks through ingesting a STIX bundle, querying the graph to understand the attack, and exporting findings for sharing or further analysis.

## Prerequisites

- Go installed (`go install github.com/aprksy/knitknot@latest`)
- A STIX 2.1 bundle (we provide [`samples/ti-sample.json`](samples/ti-sample.json))

## Step 1: Import the bundle

```bash
knitknot import --format stix-2.1 --source tutorial -f ti.gob docs/cti/samples/ti-sample.json
```

This creates `ti.gob` — a graph database with all STIX objects as nodes and relationships as edges.

## Step 2: Explore the graph

**What threat actors are in this bundle?**

```bash
knitknot query "Find('intrusion-set')" -f ti.gob
```

**Which campaign is attributed to each actor?**

```bash
knitknot query "Find('campaign').Has('attributed-to', 'Tutorial APT')" -f ti.gob
```

**What malware is used in each campaign?**

```bash
knitknot query "Find('campaign').Has('uses', 'TutorialBanker')" -f ti.gob
```

**What attack patterns (TTPs) are associated with the malware?**

```bash
knitknot query "Find('malware').Has('uses', 'Credential Dumping')" -f ti.gob
```

> Note: `Has()` matches the target by **name**. The `uses` and
> `attributed-to` verbs are multi-target — they match across node types
> (malware, attack-patterns, intrusion-sets, …).

## Step 3: Understand the attack chain

Trace the full chain actor → campaign → malware → TTP as stepwise pivots —
take the result of each query as the starting point of the next:

```bash
# 1. actor → campaign
knitknot query "Find('campaign').Has('attributed-to', 'Tutorial APT')" -f ti.gob

# 2. campaign → malware
knitknot query "Find('campaign').Has('uses', 'TutorialBanker')" -f ti.gob

# 3. malware → TTP
knitknot query "Find('malware').Has('uses', 'Credential Dumping')" -f ti.gob
```

> Note: each `.Has()` step fans out from the original `Find` node — a
> chained query like `Find('campaign').Has('uses', 'X').Has('uses', 'Y')`
> means "campaigns using both X *and* Y", not a path. For a multi-hop trace
> use `.Follow(rel, dir)` (ADR 0004), which chains from the previous node:
> `Find('intrusion-set').Follow('attributed-to','in').Follow('uses','out')`
> — direction is `'out'` (default), `'in'`, or `'both'`.

## Step 4: Export findings

**Export as Parquet for Jupyter analysis:**

```bash
knitknot export --format parquet -f ti.gob -o ti-analysis/
# → ti-analysis/nodes.parquet
# → ti-analysis/edges.parquet
```

**Export a focused subgraph as a STIX bundle:**

```bash
knitknot export --format stix --query "Find('campaign').Has('uses', 'TutorialBanker')" -f ti.gob -o focused.json
```

**Export as DOT for visualization:**

```bash
knitknot export --format dot -f ti.gob > ti-graph.dot
dot -Tsvg ti-graph.dot > ti-graph.svg  # requires graphviz
```

## Step 5: Analyze in Jupyter

```python
import pandas as pd
import networkx as nx

nodes = pd.read_parquet("ti-analysis/nodes.parquet")
edges = pd.read_parquet("ti-analysis/edges.parquet")

# Build graph
G = nx.from_pandas_edgelist(edges, "from_stix_id", "to_stix_id", "kind")

# Find most connected nodes (potential key infrastructure)
centrality = nx.degree_centrality(G)
top = sorted(centrality, key=centrality.get, reverse=True)[:10]
print(nodes[nodes.stix_id.isin(top)][["stix_id", "name", "label"]])
```

## Real-world workflow

1. **Ingest** a vendor report or MITRE ATT&CK bundle
2. **Query** to understand the attack: actors, campaigns, TTPs, infrastructure
3. **Export** as Parquet for statistical analysis in Jupyter
4. **Export** focused subgraphs as STIX bundles to share with the team
5. **Bridge** to Jupyter for clustering, anomaly detection, and detection engineering

## Next steps

- [Threat Hunting tutorial](th-tutorial.md) — use the CTI graph to hunt for attacks in your environment
- [README](README.md) — overview and quick start
