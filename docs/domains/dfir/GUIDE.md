# DFIR Case × CTI Correlation — Guide

Correlate what an investigation found against what threat intelligence
knows. You bring a CTI graph (STIX imported) and a list of case
observables; knitknot tells you which findings are known adversary
infrastructure — and who is behind them.

## Install & prerequisites

- Go installed (`go install github.com/aprksy/knitknot@latest`)
- A CTI graph: a STIX 2.1 bundle imported into a `.gob` (see the
  [CTI guide](../threat-intelligence/GUIDE.md))
- A case observable list as CSV (`type,value[,context]`) — the output of
  whatever tool produced it (EDR, SIEM, Autopsy, plaso, …)

## Quick start (2 minutes)

**1. Import the CTI reference.** We use the bundled TH sample:

```bash
knitknot import --format stix-2.1 --source mitre -f th.gob docs/cti/samples/th-sample.json
```

**2. Write a case observable list:**

```csv
type,value,context
domain,tutorial-c2.example.com,proxy log
sha256,a1b2c3d4e5f60718293a4b5c6d7e8f90a1b2c3d4e5f60718293a4b5c6d7e8f90,file scan
ipv4,203.0.113.9,firewall
```

**3. Correlate:**

```bash
knitknot correlate --observables docs/domains/dfir/samples/case-sample.csv -f th.gob
```

```
MATCH domain "tutorial-c2.example.com" (proxy log)
  indicator: TutorialBanker C2 Server
  malware:   TutorialBanker
  campaign:  Tutorial Campaign X
  actor:     Tutorial APT
  ttp:       Credential Dumping
  ttp:       Lateral Movement
MATCH file-hash-sha256 "a1b2…8f90" (file scan)
  indicator: TutorialBanker File Hash
  malware:   TutorialBanker
  campaign:  Tutorial Campaign X
  actor:     Tutorial APT
  ttp:       Credential Dumping
  ttp:       Lateral Movement
2 of 3 case observables matched
```

The `ipv4` did not match anything in the CTI graph — that's the honest
answer, not an error.

## Types and aliases

`type` is case-insensitive and accepts common aliases:

| You can write | Canonical type |
| --- | --- |
| `sha256`, `sha-256`, `file-hash-sha256` | `file-hash-sha256` |
| `sha1`, `sha-1`, `file-hash-sha1` | `file-hash-sha1` |
| `md5`, `file-hash-md5` | `file-hash-md5` |
| `domain`, `domain-name` | `domain` |
| `ip`, `ipv4`, `ipv4-addr` | `ipv4` |
| `ipv6` | `ipv6` |
| `url` | `url` |
| `filename`, `file-name`, `name` | `file-name` |

Matching is exact after canonicalization: domains are lowercased with a
trailing dot stripped, hashes are lowercased. So `Tutorial-C2.Example.COM.`
matches `tutorial-c2.example.com`.

## JSON output

```bash
knitknot correlate --observables case.csv -f th.gob --format json
```

Emits an array of matches with the observable, context, and every
`id/label/name/stix_id` reference — pipe it into your report tooling.

## Build a persistable case workspace

A report is one-shot. A **workspace** is a graph you can keep, query, and
hand to Jupyter:

```bash
knitknot correlate --observables case.csv -f th.gob --workspace case-workspace.gob
```

This writes a thin, self-contained graph containing:

- the case observables,
- the indicators that matched them,
- the reachable malware / campaigns / actors / TTPs,
- the `based-on` links and the connecting context edges.

The source graph (`th.gob`) is never modified.

Query it like any graph:

```bash
knitknot query "Find('observable')" -f case-workspace.gob
knitknot query "Find('indicator').Has('based-on', 'tutorial-c2.example.com')" -f case-workspace.gob
knitknot query "Find('malware')" -f case-workspace.gob
```

## Analyze in Jupyter

```bash
knitknot export --format parquet -f case-workspace.gob -o case-parquet/
```

```python
import pandas as pd, networkx as nx
nodes = pd.read_parquet("case-parquet/nodes.parquet")
edges = pd.read_parquet("case-parquet/edges.parquet")
G = nx.from_pandas_edgelist(edges, "from_stix_id", "to_stix_id", "kind")
# cluster the observable layer, rank adversary infrastructure, etc.
```

## What's NOT there yet

- **No time.** No event timestamps, no time-range queries. Timeline work
  stays in your DFIR tool or Jupyter.
- **Only indicator patterns are correlated.** Observables that exist only
  as standalone SCO nodes aren't consulted.
- **Only the main chain is walked** (`indicates` / `uses` / `attributed-to`);
  `communicates-with` / `targets` are ignored.
- **Exact match only** — no fuzzy/substring matching.
- **Workspace is `.gob` only**; no `bolt:` output yet.

## Where to go next

- Honest limits and status: [STATUS.md](STATUS.md)
- Domain blueprint: [README.md](README.md)
- The CTI side: [Threat Intelligence guide](../threat-intelligence/GUIDE.md)
