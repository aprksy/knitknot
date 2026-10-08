# Threat Hunting (TH) Tutorial

Use the CTI graph to form hunting hypotheses, extract attack patterns, and bridge to Jupyter for operational analysis.

## Prerequisites

- Go installed (`go install github.com/aprksy/knitknot@latest`)
- A STIX 2.1 bundle (we provide [`samples/th-sample.json`](samples/th-sample.json))
- Jupyter with pandas + networkx (for the analysis step)

## The hunting workflow

```
1. Form hypothesis    → "I think this group is using technique X"
2. Understand pattern → knitknot: query the CTI graph for technique X
3. Extract indicators  → knitknot: export IOCs/infrastructure as Parquet
4. Hunt operationally  → Jupyter/SIEM: search for those patterns in live data
5. Build detection     → Jupyter: turn findings into detection rules
```

## Step 1: Import the CTI reference bundle

```bash
knitknot import --format stix-2.1 --source mitre -f th.gob docs/cti/samples/th-sample.json
```

This is your curated threat intelligence reference — the "what does this attack look like" knowledge base.

## Step 2: Form a hypothesis

**"I think the adversary is using credential dumping (T1003) as part of their lateral movement."**

Query the CTI graph to understand what this looks like:

```bash
# What attack patterns match credential dumping? (Where uses exact '=' match)
knitknot query "Find('attack-pattern').Where('n.name', '=', 'Credential Dumping')" -f th.gob

# What malware uses this technique?
knitknot query "Find('malware').Has('uses', 'Credential Dumping')" -f th.gob

# What indicators point at that malware?
knitknot query "Find('indicator').Has('indicates', 'TutorialBanker')" -f th.gob

# Which campaign and actor sit behind it?
knitknot query "Find('campaign').Has('attributed-to', 'Tutorial APT')" -f th.gob
```

## Step 3: Extract the attack pattern

Export the subgraph for this hypothesis:

```bash
knitknot export --format stix --query "Find('malware').Has('uses', 'Credential Dumping')" -f th.gob -o credential-dumping.json
```

This gives you a focused STIX bundle with the malware → TTP pair and their
relationship — the pattern you're hunting for.

## Step 4: Export IOCs for operational hunting

```bash
knitknot export --format parquet -f th.gob -o th-analysis/
# → th-analysis/nodes.parquet
# → th-analysis/edges.parquet
```

## Step 5: Hunt in Jupyter

Load the CTI graph and your SIEM data, then search for matches:

```python
import pandas as pd
import networkx as nx

# Load CTI reference
nodes = pd.read_parquet("th-analysis/nodes.parquet")
edges = pd.read_parquet("th-analysis/edges.parquet")

# Load SIEM alerts (your operational data)
alerts = pd.read_csv("siem-alerts.csv")

# Extract IOCs from CTI
iocs = nodes[nodes.label == "indicator"][["stix_id", "name", "props_json"]]

# Hunt: find alerts matching CTI IOCs
# (example: match on file hash)
for _, ioc in iocs.iterrows():
    props = pd.json_normalize(ioc["props_json"])
    if "pattern" in props.columns:
        # Extract hash from STIX pattern
        pattern = props["pattern"].iloc[0]
        # Search SIEM alerts for this hash
        matches = alerts[alerts["file_hash"].str.contains(pattern, na=False)]
        if len(matches) > 0:
            print(f"MATCH: {ioc['name']} → {len(matches)} alerts")
```

## Step 6: Build a detection

Once you've confirmed a match, formalize it into a detection rule:

```python
# Example: Sigma rule from confirmed IOC
rule = """
title: Known Malicious File Hash
id: <uuid>
status: experimental
description: Detects file hash from CTI campaign X
references:
    - https://attack.mitre.org/techniques/T1003/
logsource:
    category: file_creation
detection:
    selection:
        file_hash: "<hash-from-CTI>"
    condition: selection
falsepositives:
    - Legitimate administrative tools
level: high
"""
```

## Real-world hunting loop

1. **Ingest** new threat intelligence → update the CTI graph
2. **Query** the graph to understand the adversary's TTPs
3. **Export** the attack pattern as Parquet/STIX
4. **Hunt** in Jupyter: join CTI patterns with SIEM alerts
5. **Confirm** matches, build detections
6. **Repeat** as new intelligence arrives

## Key insight

Knitknot is the **advisor** in the hunt — it doesn't do the hunting, but it tells the hunter what to look for. The actual hunting (searching live data) happens in Jupyter/SIEM, but the *pattern* being hunted comes from Knitknot's CTI analysis.

## Next steps

- [Threat Intelligence tutorial](ti-tutorial.md) — analyze published attacks
- [README](README.md) — overview and quick start
