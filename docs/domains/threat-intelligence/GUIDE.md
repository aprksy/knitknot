<!-- ext: cti-guide -->
# CTI Extension Guide

## 1. What it is

knitknot's threat-intelligence extension is a STIX 2.1 import/export bridge into knitknot's embedded property graph: STIX objects become nodes, STIX relationships become edges, and unknown `x-*` types are preserved verbatim as generic nodes instead of being dropped. Once imported, you query everything with the knitknot DSL (`Find(...).Has(...).Where(...)`), and re-importing an updated bundle appends version history instead of overwriting.

Design rationale lives in [README.md](README.md); the candid snapshot of what works today and what doesn't lives in [STATUS.md](STATUS.md) — this guide won't repeat either, it just gets you running.

## 2. Install & prerequisites

```bash
go install github.com/aprksy/knitknot@latest
# or: git clone https://github.com/aprksy/knitknot && cd knitknot && go build -o knitknot .
```

- Go 1.25+ (see `go.mod`).
- Optional: Graphviz (`dot`) if you want SVG exports in beat 5.
- Confirm the binary is on PATH:

```bash
knitknot --help
```

> Always pass `-f <file.gob`. Without it the import runs, prints success, and evaporates with the process.

## 3. Quick start (60 seconds)

Save this as `bundle.json` — one indicator, one malware, one relationship:

```json
{
  "type": "bundle",
  "id": "bundle--aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
  "spec_version": "2.1",
  "objects": [
    {
      "type": "indicator",
      "spec_version": "2.1",
      "id": "indicator--a1a1a1a1-a1a1-a1a1-a1a1-a1a1a1a1a1a1",
      "created": "2024-03-08T09:00:00.000Z",
      "modified": "2024-03-08T09:00:00.000Z",
      "name": "Demo payload hash",
      "pattern": "[file:hashes.'SHA-256' = 'e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855']",
      "pattern_type": "stix",
      "valid_from": "2024-03-08T09:00:00.000Z"
    },
    {
      "type": "malware",
      "spec_version": "2.1",
      "id": "malware--b2b2b2b2-b2b2-b2b2-b2b2-b2b2b2b2b2b2",
      "created": "2024-03-07T15:30:00.000Z",
      "modified": "2024-03-07T15:30:00.000Z",
      "name": "DemoRat",
      "is_family": false
    },
    {
      "type": "relationship",
      "spec_version": "2.1",
      "id": "relationship--11111111-1111-1111-1111-111111111111",
      "created": "2024-03-10T12:00:00.000Z",
      "modified": "2024-03-10T12:00:00.000Z",
      "relationship_type": "indicates",
      "source_ref": "indicator--a1a1a1a1-a1a1-a1a1-a1a1-a1a1a1a1a1a1",
      "target_ref": "malware--b2b2b2b2-b2b2-b2b2-b2b2-b2b2b2b2b2b2"
    }
  ]
}
```

Then:

```bash
knitknot import --format stix-2.1 --source my-feed -f ti.gob bundle.json
# what just happened: staged + validated the bundle, then upserted 2 nodes + 1 edge into ti.gob, tagged with source_feed=my-feed.

knitknot query "Find('indicator')" -f ti.gob
# what just happened: listed every indicator node (expect: Demo payload hash (indicator)).

knitknot export --format stix -f ti.gob > out.json
# what just happened: wrote the graph back out as a STIX 2.1 bundle (only stix_id-carrying nodes are emitted).
```

All three commands verified against the real CLI.

## 4. The 5-beat demo (10 minutes)

For the full live run: `bash demo/cti-demo.sh` from the repo root (needs Go + python3; Graphviz optional). It builds the real binary to `/tmp/knitknot-demo` and drives it through five beats on `extensions/cti/testdata/demo-campaign.json`. Summary:

**Beat 1 — Ingest.**

```bash
knitknot import --format stix-2.1 --source northwind-cert -f demo.gob extensions/cti/testdata/demo-campaign.json
```

Expect `-- Imported ... as stix-2.1` plus `-- Saved to demo.gob`. *What this proves: a CERT bundle lands in the graph, idempotently, with provenance attached.*

**Beat 2 — Investigate.** Three DSL queries (`Has()` follows outgoing edges):

```bash
knitknot query "Find('campaign').Has('uses', 'frostbiterat')" -f demo.gob
knitknot query "Find('indicator').Has('indicates', 'frostbiterat')" -f demo.gob
knitknot query "Find('indicator').Where('n.pattern_type', '=', 'stix')" -f demo.gob
```

Expect `RESULT (text):` rows like `n: operation-night-harvest (campaign)` + `v0: frostbiterat (malware)`, then three `indicates` hits, then five `pattern_type=stix` indicators. *What this proves: campaign→malware and indicator→malware pivots work through the normal DSL.*

**Beat 3 — Evolve.** The script tweaks the bundle with python3 (indicator `...e5e5...` confidence → 85, indicator `...f6f6...` revoked) and re-imports under a new feed:

```bash
knitknot import --format stix-2.1 --source first-response-ir -f demo.gob $EVOLVED
```

Expect another clean `-- Imported` line. *What this proves: same import line, new snapshot — props merge, history appends, revoked stays addressable.*

**Beat 4 — History.** Graph IDs (`n1`, `n2`, …) are storage-assigned, so resolve the STIX ID first, then inspect:

```bash
IND_ID=$(knitknot query --format json "Find('indicator').Where('n.stix_id', '=', 'indicator--e5e5e5e5-e5e5-e5e5-e5e5-e5e5e5e5e5e5')" -f demo.gob | python3 -c "import sys, json; raw = sys.stdin.read(); print(json.loads(raw[raw.index('['):])[0]['n']['id'])")
knitknot history "$IND_ID" -f demo.gob
```

Expect one line per snapshot, newest-last:

```text
-- History for n6 (2 snapshots)
rev=6 time=2026-09-23T16:01:03+07:00 source=northwind-cert deleted=false props={confidence=60, ..., source_feed=northwind-cert, ...}
rev=31 time=... source=first-response-ir deleted=false props={confidence=85, ..., source_feed=first-response-ir, ...}
```

Formally: `rev=N time=<RFC3339> source=<feed> deleted=<bool> props={<sorted k=v>}` (see `cmd/history.go`). *What this proves: both claims coexist with their sources — the update didn't destroy the original.*

**Beat 5 — Share.** Round-trip for partners, pictures for humans:

```bash
knitknot export --format stix -f demo.gob -o demo-stix.json
knitknot export --format dot -f demo.gob > demo.dot
dot -Tsvg demo.dot -o demo.svg   # only if graphviz is installed; the script skips otherwise
```

Expect a STIX bundle in `demo-stix.json`, a `digraph KnitKnot {` DOT file, and (if `dot` exists) `demo.svg`. *What this proves: the graph leaves the building as STIX or visuals.*

Demo summary also lives in [demo/README.md](../../../demo/README.md).

## 5. Data model cheat-sheet

| STIX type | Node label | Key props | Sample |
| --- | --- | --- | --- |
| `indicator` | `indicator` | `pattern`, `pattern_type`, `valid_from`, `valid_until` | `pattern=[file:hashes.'SHA-256' = '…']` |
| `malware` | `malware` | `name`, `is_family`, `malware_types` | `name=FrostbiteRAT` |
| `threat-actor` | `threat-actor` | `name`, `created`, `modified` | `name=Crimson Dune` |
| `campaign` | `campaign` | `name`, `created`, `modified` | `name=operation-night-harvest` |
| `identity` | `identity` | `name`, `identity_class` | `name=Northwind CERT` |
| `vulnerability` | `vulnerability` | `name` (CVE), `external_references` | `name=CVE-2021-44228` |
| `relationship` | edge, kind = `relationship_type` | `source_ref` → `target_ref`, `start_time`/`stop_time` | `indicates`, `uses`, `targets`, `attributed-to`, `based-on`, … |
| `sighting` / `marking-definition` / `report` / `observed-data` | own label (`sighting`, `report`, …) / generic node | type-specific props + provenance | preserved, queryable via `Find('<label>')` |
| custom `x-*` | generic node, original `type` as label | all raw props incl. unknown top-level fields | `x-custom-thing` + `custom_prop=preserved-for-round-trip` |
| every node & edge | — | `stix_id`, `type`, `source_feed`, `imported_at` (+ `confidence`, `labels`, markings when present) | `source_feed=northwind-cert` |

Labels and edge kinds come from `extensions/cti/vocabulary.go` (11 labels, 8 edge kinds).

**Identity:** `stix_id` is the stable key — re-importing the same STIX `id` merges into the same node/edge instead of duplicating. Versions coexist: `(created, modified)` distinguish them and history keeps each snapshot, so an update is a merge + a new history entry, never a silent overwrite.

## 6. Common recipes

- **"Find all malware used by a campaign"** — `Has` chain over the `uses` edge (outgoing from the campaign):
  ```bash
  knitknot query "Find('campaign').Has('uses', 'frostbiterat')" -f demo.gob
  ```
- **"Find indicators about to expire"** — honestly: the DSL has no date-range ops. `>`/`<` only coerce numbers (`pkg/query/default.go` `compare()` → `toFloat`; date strings never parse, so `Where('n.valid_from', '>', '2024-…')` matches nothing — verified). `=`/`!=` on exact timestamp strings work, and numeric props like `confidence` support `>`/`<`. For real expiry triage, export and filter:
  ```bash
  knitknot query "Find('indicator').Where('n.valid_until', '=', '2024-06-08T09:00:00.000Z')" -f demo.gob
  knitknot export --format stix -f demo.gob | jq '.objects[] | select(.type=="indicator") | {id, valid_until}'
  ```
- **"Check whether re-importing changed my graph"** — re-import is state-idempotent (same nodes/edges; safe to re-run a feed pull), and `history` shows whether a new snapshot landed:
  ```bash
  knitknot import --format stix-2.1 --source my-feed -f ti.gob bundle.json
  knitknot history <node-id> -f ti.gob
  ```
- **"Audit who supplied a claim"** — every node/edge carries `source_feed` from `--source`, so filter on it (nodes) or export + `jq` (edges included):
  ```bash
  knitknot query "Find('indicator').Where('n.source_feed', '=', 'northwind-cert')" -f demo.gob
  ```
- **"What was this indicator before the update?"** — resolve STIX ID → graph ID, then read the snapshots (beat 4 pattern):
  ```bash
  knitknot history "$IND_ID" -f demo.gob
  ```

## 7. What's NOT there yet

- **No TLP enforcement** — markings are stored as props, never filtered on.
- **No pattern matching** — indicator `pattern` is an opaque string; nothing compiles or matches it.
- **No TAXII** — no client/server, no polling, no enrichment; feed plumbing is yours.
- **No point-in-time DSL query** — no `.AsOf(...)`; time travel is kernel primitives (`GetNodeAt`/`GetNodeHistory`) only.
- **Edge history not at the CLI** — `history` takes a node ID; edge versions exist in the kernel but aren't exposed.
- **Sync is gob→gob only** — gob→sqlite errors out (`sync not implemented for gob -> sqlite yet`).
- **Subgraph versioning not implemented** — history is per-node, not per-subgraph.
- **Analyst-sized only** — comfortable in the low thousands of objects; tens of thousands uncharted, no benchmarks.

Details in [STATUS.md](STATUS.md).

## 8. Where to go next

- Live run: `bash demo/cti-demo.sh` + [demo/README.md](../../../demo/README.md)
- Honest limits: [STATUS.md](STATUS.md)
- Design blueprint: [README.md](README.md)
- Building your own domain: [docs/domains/README.md](../README.md) (extension rules, admission checklist)
