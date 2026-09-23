#!/usr/bin/env bash
# demo/cti-demo.sh — self-contained Threat Intelligence demo for knitknot.
#
# Drives the real CLI through a 5-beat narrative on the demo-campaign fixture:
# ingest a STIX 2.1 bundle, investigate via the DSL, evolve (re-import an
# updated bundle), inspect version history, share (STIX + DOT/SVG export).
#
# Run from the repo root:  bash demo/cti-demo.sh
#
# Cleanup guidance — artifacts this script creates (all removable, the script
# is the source of truth; the temp bundle removes itself on exit):
#   /tmp/knitknot-demo   go build output
#   demo.gob             graph file (beats 1-5 read/write this)
#   $EVOLVED (mktemp)    temp updated bundle, auto-removed via trap
#   demo-stix.json       STIX export (beat 5)
#   demo.dot             DOT export (beat 5)
#   demo.svg             SVG render (beat 5, only if graphviz is installed)
# rm -f /tmp/knitknot-demo demo.gob demo-stix.json demo.dot demo.svg
set -euo pipefail

[ -f go.mod ] || { echo "run from the repo root: bash demo/cti-demo.sh" >&2; exit 1; }
command -v go >/dev/null || { echo "need Go on PATH" >&2; exit 1; }
command -v python3 >/dev/null || { echo "need python3 on PATH" >&2; exit 1; }

KN=/tmp/knitknot-demo
GOB=demo.gob
FIXTURE=extensions/cti/testdata/demo-campaign.json
EVOLVED=$(mktemp /tmp/cti-evolved.XXXXXX.json)
trap 'rm -f "$EVOLVED"' EXIT

# demo: build the real binary once; every beat below drives this CLI.
go build -o "$KN" .

# demo: Beat 1 — Ingest. A CERT shares a STIX 2.1 bundle; idempotent import
# stages+validates before mutating, then persists to demo.gob.
echo "── Beat 1: Ingest"
rm -f "$GOB"
"$KN" import --format stix-2.1 --source northwind-cert -f "$GOB" "$FIXTURE"

# demo: Beat 2 — Investigate. Has() follows OUTGOING edges; each verb resolves
# its target label via the registry (uses->malware, indicates->malware).
echo "── Beat 2: Investigate"
echo "-- which campaign stages frostbiterat?"
"$KN" query "Find('campaign').Has('uses', 'frostbiterat')" -f "$GOB"
echo "-- which indicators point at frostbiterat?"
"$KN" query "Find('indicator').Has('indicates', 'frostbiterat')" -f "$GOB"
echo "-- all STIX-pattern indicators (Where filter demo)"
"$KN" query "Find('indicator').Where('n.pattern_type', '=', 'stix')" -f "$GOB"

# demo: Beat 3 — Evolve. IR re-shares the bundle: indicator hash-01 confidence
# set to 85, hash-02 revoked. Re-import under a new --source merges props and
# appends history instead of overwriting — same import line, new snapshot.
echo "── Beat 3: Evolve"
python3 - "$FIXTURE" "$EVOLVED" <<'EOF'
import sys
import json
src, dst = sys.argv[1], sys.argv[2]
bundle = json.load(open(src))
for obj in bundle["objects"]:
    if obj.get("id") == "indicator--e5e5e5e5-e5e5-e5e5-e5e5-e5e5e5e5e5e5":
        obj["confidence"] = 85
    if obj.get("id") == "indicator--f6f6f6f6-f6f6-f6f6-f6f6-f6f6f6f6f6f6":
        obj["revoked"] = True
json.dump(bundle, open(dst, "w"), indent=1)
EOF
"$KN" import --format stix-2.1 --source first-response-ir -f "$GOB" "$EVOLVED"

# demo: Beat 4 — History. Graph node IDs (n1, n2, …) are storage-assigned, so
# resolve the STIX ID to its graph ID first; history then shows both snapshots
# (northwind-cert without confidence → first-response-ir with confidence=85).
echo "── Beat 4: History"
IND_ID=$("$KN" query --format json \
  "Find('indicator').Where('n.stix_id', '=', 'indicator--e5e5e5e5-e5e5-e5e5-e5e5-e5e5e5e5e5e5')" \
  -f "$GOB" | python3 -c "import sys, json; raw = sys.stdin.read(); print(json.loads(raw[raw.index('['):])[0]['n']['id'])")
echo "-- bumped indicator lives at graph node $IND_ID"
"$KN" history "$IND_ID" -f "$GOB"

# demo: Beat 5 — Share. Round-trip back to STIX for partners, DOT/SVG for humans.
echo "── Beat 5: Share"
"$KN" export --format stix -f "$GOB" -o demo-stix.json
head -c 200 demo-stix.json; echo
"$KN" export --format dot -f "$GOB" > demo.dot
if command -v dot >/dev/null 2>&1; then
  dot -Tsvg demo.dot -o demo.svg
  echo "-- wrote demo.svg"
else
  echo "-- graphviz 'dot' not installed; skipping SVG (demo.dot kept)"
fi

echo "── Done: demo.gob + demo-stix.json + demo.dot ready (see header for cleanup)"
