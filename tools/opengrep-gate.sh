#!/usr/bin/env bash
# opengrep gate.
#
# Two things fail the build:
#   1. any ERROR-severity finding
#   2. a WARNING whose rule is in the BLOCKING_WARNINGS set below
#
# WHY (2) EXISTS -- and it is not optional for a TypeScript repo:
# severity is a property of the RULE, not of how dangerous the code is. Measured
# on clichse, `eval()` and `innerHTML` assignment are WARNING-severity rules. An
# ERROR-only gate passed a file containing both, while reporting them as "not
# gating". A gate that is blind to the entire frontend because of a registry's
# severity labelling is worse than no gate -- it certifies the code it cannot see.
#
# WHY THE SPLIT AT ALL: bare `opengrep scan` exits 0 regardless of findings.
# Measured: securai2 produced 97 ERROR findings and exit code 0. So the scan is
# run per severity tier and the tiers are judged separately.
#
# Rules are VENDORED (tools/opengrep/rules.yml) instead of `--config p/default`,
# which re-downloads from semgrep.dev on every run: network dependency in CI, a
# supply-chain surface, and silent drift when the registry changes without a
# commit. Bump the pack deliberately and review the diff.
set -uo pipefail

RULES="tools/opengrep/rules.yml"
OUT="${TMPDIR:-/tmp}/opengrep-$RANDOM.json"

if ! command -v opengrep >/dev/null 2>&1; then
  echo "opengrep not found on PATH." >&2
  echo "This gate requires it (pinned install, not AUR): see tools/opengrep/README.md" >&2
  exit 3
fi
if [ ! -f "$RULES" ]; then
  echo "vendored rule pack missing: $RULES" >&2
  exit 3
fi

# Rules whose WARNING severity must still fail the build. Keep this list short,
# justified, and language-driven -- it is the only place a warning can block.
BLOCKING_WARNINGS=(
  # --- TypeScript / JavaScript: the registry files these as WARNING ---
  'detected-eval'                 # eval() of dynamic input
  'eval-detected'                 # eval() of dynamic input (TS rule id)
  'react-dangerouslysetinnerhtml'
  'no-dangerouslysetinnerhtml'
  'raw-html-format'
  'innerhtml'                     # DOM XSS sink
  'detect-non-literal-regexp'     # ReDoS
  'detect-child-process'          # command execution from JS
  'no-shell-command'              # shell invocation from JS
  # --- cross-language ---
  'detected-hardcoded-secret'
  'hardcoded-credentials'
  'no-hardcoded-password'
)

# Files/dirs that are generated, vendored, or not ours. Scanning them produces
# findings nobody can act on, which is how a gate gets ignored.
EXCLUDES=(
  --exclude node_modules
  --exclude vendor
  --exclude .venv
  --exclude dist
  --exclude build
  --exclude '*.min.js'
  --exclude '*.min.css'
  # The vendored pack is JSON full of secret-shaped literals (the very patterns
  # these rules look for), so scanning it flags itself. It is data, not code.
  --exclude tools/opengrep
  # Tooling: this script and its siblings describe the very patterns being
  # detected, so scanning them is the scanner reading its own rulebook.
  --exclude tools/opengrep-gate.sh
  # Markdown prose: spec and contract documents describe protocols and API shapes
  # rather than implementing them, and markdown cannot carry a nosemgrep. Scanning
  # it produced an insecure-websocket finding on a requirement document.
  --exclude 'specs/**/*.md'
  --exclude '**/contracts/**/*.md'
  --exclude 'docs/**/*.md'
  --exclude '*.md'
  # Dev seed data: hand-written fixtures whose bcrypt hashes are the documented
  # dev password ('clichse-dev'), not credentials. Scanning them produces 11
  # identical findings nobody can act on.
  --exclude contracts/dev/99_seed.sql
  # Documentation quoting the dev seed credentials. Markdown cannot carry a
  # nosemgrep, and the value is a documented dev fixture in the same file.
  --exclude docs/SEED_REQUIREMENTS.md
)

echo "opengrep: $(opengrep --version 2>/dev/null | head -1)"
echo "rules: $RULES ($(python3 -c "import json;print(len(json.load(open('$RULES')).get('rules') or []))" 2>/dev/null || echo '?') rules)"

# Pass 1: ERROR severity gates.
opengrep scan --config "$RULES" --disable-version-check --quiet \
  --severity ERROR --json --output "$OUT" "${EXCLUDES[@]}" . >/dev/null 2>&1
RC1=$?

# Pass 2: WARNING severity is reported, and a subset gates.
opengrep scan --config "$RULES" --disable-version-check --quiet \
  --severity WARNING --json --output "${OUT%.json}-warn.json" "${EXCLUDES[@]}" . >/dev/null 2>&1 || true

# Judge. Exit code comes from this python, not from opengrep, because the
# blocking-warning set lives here.
python3 - "$OUT" "${OUT%.json}-warn.json" "${BLOCKING_WARNINGS[@]}" <<'PY'
import json, sys
from collections import Counter

def load(p):
    try:
        return json.load(open(p)).get("results") or []
    except Exception:
        return []

errs = load(sys.argv[1])
warns = load(sys.argv[2])
blocking_ids = set(sys.argv[3:])

def rule_short(check_id):
    return check_id.split(".")[-1]

def summarise(title, rows, limit=15):
    print(f"\n### {title}: {len(rows)}")
    if not rows:
        return
    for check, n in Counter(r.get("check_id", "?") for r in rows).most_common(limit):
        print(f"  {n:4}  {check}")
    print("  locations:")
    for r in rows[:limit]:
        print(f"    {r.get('path')}:{r.get('start', {}).get('line')}  {rule_short(r.get('check_id', ''))}")

summarise("ERROR (gates the build)", errs)

blocking = [w for w in warns if rule_short(w.get("check_id", "")) in blocking_ids]
advisory = [w for w in warns if rule_short(w.get("check_id", "")) not in blocking_ids]

if blocking:
    summarise("WARNING promoted to blocking (see BLOCKING_WARNINGS)", blocking)
summarise("WARNING (reported only)", advisory)

# Files the scanner could not parse are unscanned, so the gate is not evidence
# about them. Say so loudly rather than implying full coverage.
parse_errs = set()
for p in sys.argv[1:3]:
    try:
        for e in (json.load(open(p)).get("errors") or []):
            for t in (e.get("type") or [])[1:]:
                for item in t:
                    if isinstance(item, dict) and item.get("path"):
                        parse_errs.add(item["path"])
    except Exception:
        pass
if parse_errs:
    print(f"\n!! {len(parse_errs)} file(s) could not be parsed and were NOT scanned:")
    for f in sorted(parse_errs)[:10]:
        print(f"   {f}")

if errs or blocking:
    print(f"\nFAIL: {len(errs)} ERROR + {len(blocking)} blocking WARNING finding(s).", file=sys.stderr)
    sys.exit(1)

print("\nPASS: no blocking findings.")
PY
exit $?
