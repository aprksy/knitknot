#!/usr/bin/env bash
# Capture the exact ruleset that `--config p/default` resolves to, as a vendored
# pack. Run this on a host with opengrep when bumping the rules.
#
# Why capture instead of pinning a version string: the registry has no released
# version identifier for a ruleset, and `p/default` is a moving target. The only
# reproducible artifact is the resolved pack itself, so that is what gets pinned
# and reviewed.
#
# Why the polling loop: opengrep writes the resolved rules to /tmp/semgrep-*.rules
# and deletes them when the scan exits, so the file has to be grabbed while the
# scan is still running.
#
# usage: capture-pack.sh <out.json> [target-dir]
set -uo pipefail

OUT="${1:?usage: capture-pack.sh <out.json> [target-dir]}"
TARGET="${2:-.}"

command -v opengrep >/dev/null 2>&1 || {
  echo "opengrep not on PATH. Install it first: tools/install-opengrep.sh" >&2
  exit 1
}

rm -f "$OUT" /tmp/semgrep-*.rules 2>/dev/null
opengrep scan --config p/default --disable-version-check --quiet "$TARGET" >/dev/null 2>&1 &
SCAN=$!

for _ in $(seq 1 600); do
  for f in /tmp/semgrep-*.rules; do
    [ -f "$f" ] || continue
    if [ ! -s "$OUT" ]; then
      cp "$f" "$OUT" 2>/dev/null && echo "captured $f -> $OUT"
    fi
  done
  kill -0 "$SCAN" 2>/dev/null || break
  sleep 0.05
done
wait "$SCAN" 2>/dev/null
rm -f /tmp/semgrep-*.rules 2>/dev/null

[ -s "$OUT" ] || { echo "FAILED to capture the resolved pack" >&2; exit 1; }

python3 - "$OUT" <<'PY'
import json, sys
d = json.load(open(sys.argv[1]))
rules = d.get("rules") or []
print(f"rules in pack: {len(rules)}")
assert rules, "pack has no rules"
PY
echo "ok: $OUT ($(du -h "$OUT" | cut -f1))"
