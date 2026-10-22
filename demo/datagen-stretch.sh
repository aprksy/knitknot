#!/usr/bin/env bash
# Stretch driver (ADR 0009): self-contained. Generates a synthetic
# dataset as write-DSL, streams it through one fresh REPL session with
# verification queries appended, then asserts exact expected counts.
# Fails loudly (nonzero) on any mismatch or mid-stream write error.
#
# Usage: demo/datagen-stretch.sh [--nodes N] [--seed S]
set -euo pipefail
cd "$(dirname "$0")/.."

NODES=3000; SEED=7
while [[ $# -ge 1 ]]; do
  case "$1" in
    --nodes) NODES="$2"; shift 2 ;;
    --seed)  SEED="$2";  shift 2 ;;
    *) echo "unknown arg: $1" >&2; exit 1 ;;
  esac
done

DSL=$(mktemp /tmp/knitknot-stretch.XXXXXX.txt); OUT=$(mktemp /tmp/knitknot-stretch-out.XXXXXX.txt)
trap 'rm -f "$DSL" "$OUT"' EXIT

echo "=== build ==="
make build >/dev/null

echo "=== generate (seed=$SEED nodes=$NODES) ==="
python3 demo/datagen.py --nodes "$NODES" --seed "$SEED" > "$DSL"
ADDS=$(grep -c '^ADDNODE' "$DSL"); CONNECTS=$(grep -c '^CONNECT' "$DSL"); UPDATES=$(grep -c '^UPDATE' "$DSL")
echo "statements: $ADDS add + $CONNECTS connect + $UPDATES update"

# Verification queries ride the SAME session, after all writes.
{
  echo "DEFINE has_skill TO Skill VIA name"
  echo "Find('Person')"
  echo "Find('Company')"
  echo "Find('Skill')"
  echo "Find('Person').Has('has_skill', 'skill000')"
  echo "Find('Person').Where('n.verified', '=', 'true')"
  echo "Find('Person').Where('n.city', '=', 'city99')"
} >> "$DSL"

echo "=== stream through the REPL ==="
START=$(date +%s%N)
./knitknot repl < "$DSL" > "$OUT" 2> /tmp/knitknot-stretch.err || true
dur=$(( ($(date +%s%N) - START) / 1000000 ))
echo "wall: ${dur}ms for $((ADDS + CONNECTS + UPDATES + 7)) statements"

FAILS=$(grep -c '^Error:' "$OUT" || true)
if [[ "$FAILS" != 0 ]]; then
  echo "FAIL: $FAILS unexpected errors during the stream:" >&2
  grep '^Error:' "$OUT" | head -5 >&2
  exit 1
fi

echo "=== verify (exact expected counts) ==="
# QUERY_PATH=(Person, Company, Skill, Has, verified, city99). 0-row
# queries print "(no results)" instead of "-- 0 result(s)" — map to 0.
readarray -t COUNTS < <(grep -E -- '-- [0-9]+ result\(s\)|^\(no results\)' "$OUT" | sed 's/^-- //;s/ result(s)//;s/^(no results)$/0/')
python3 - "$NODES" "${COUNTS[@]}" <<'EOF'
import sys
nodes = int(sys.argv[1])
persons, companies, skills, has, verified, empty = [int(x) for x in sys.argv[2:8]]
expected = {
    "persons": nodes, "companies": 30, "skills": 25,
    "has_skill->skill000": None,  # only asserts > 0 below
}
fail = 0
def check(name, got, want=None, at_least=None):
    global fail
    ok = (want is None or got == want) and (at_least is None or got >= at_least)
    print(f"  {name}: got {got}, want {want if want is not None else '>=' + str(at_least)}", "OK" if ok else "FAIL")
    if not ok:
        fail = 1
check("Person rows", persons, nodes)
check("Company rows", companies, 30)
check("Skill rows", skills, 25)
check("Has('has_skill','skill000')", has, at_least=1)
check("verified=true", verified, (nodes + 16) // 17)  # UPDATE stride
check("city99 (negative probe)", empty, 0)
sys.exit(fail)
EOF
RC=$?
[[ $RC -eq 0 ]] && echo "=== PASS (${dur}ms) ===" || echo "=== FAIL ==="
exit $RC
