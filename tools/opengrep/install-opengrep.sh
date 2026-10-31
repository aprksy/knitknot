#!/usr/bin/env bash
# Install the opengrep static-analysis gate into a repo.
#
# Design decisions this encodes, each learned the hard way:
#
# 1. A STANDALONE workflow, not a step bolted onto an existing job. Repos here
#    split CI across several path-filtered workflows (clichse has three); adding
#    a step to each would run the scan repeatedly on some pushes and not at all
#    on others. One unfiltered workflow covers the tree exactly once.
#
# 2. Rules are VENDORED and trimmed to the languages the repo actually contains,
#    derived at install time. `--config p/default` re-downloads from semgrep.dev
#    on every run: network dependency in CI, supply-chain surface, and silent
#    drift when the registry changes without a commit.
#
# 3. `--severity ERROR --error` is the gate. Bare `opengrep scan` exits 0
#    regardless of findings (measured: 97 ERROR findings, exit 0 on securai2).
#
# 4. The scan runs through tools/opengrep-gate.sh, so the two-tier behaviour and
#    the coverage caveat live in one place per repo rather than in YAML.
#
# 5. The full source pack lives on the HOST (/opt/opengrep/rules-full.json), not
#    in the repo: it is 2.2 MB and only needed when regenerating, so committing
#    it would bloat every clone for no runtime benefit.
#
# usage: install-opengrep.sh <repo-root> [runs-on-label]
set -euo pipefail

ROOT="${1:?usage: install-opengrep.sh <repo-root> [runs-on-label]}"
cd "$ROOT"

git rev-parse --git-dir >/dev/null 2>&1 || { echo "not a git repo: $ROOT" >&2; exit 1; }

REPO_NAME="$(basename "$(git rev-parse --show-toplevel)")"
LABEL="${2:-$REPO_NAME}"

# Integration branch: prefer the branch currently checked out, else dev, else main.
BRANCH="$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo main)"
case "$BRANCH" in
  main|dev) ;;
  *) if git show-ref --verify -q refs/heads/dev; then BRANCH=dev; else BRANCH=main; fi ;;
esac
OTHER=main; [ "$BRANCH" = main ] && OTHER=dev

SRC="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Locate the source pack: $OPENGREP_FULL_PACK, beside this script, or the host install.
FULL_PACK="${OPENGREP_FULL_PACK:-}"
for cand in "$SRC/rules-full.json" /opt/opengrep/rules-full.json; do
  [ -n "$FULL_PACK" ] && break
  [ -f "$cand" ] && FULL_PACK="$cand"
done

if [ -z "$FULL_PACK" ] || [ ! -f "$FULL_PACK" ]; then
  cat >&2 <<EOF
missing the full rules pack (looked for rules-full.json beside this script and at
/opt/opengrep/rules-full.json).

Create it once on a host with opengrep:
  bash tools/opengrep/capture-pack.sh /opt/opengrep/rules-full.json .
or point at an existing copy:
  OPENGREP_FULL_PACK=/path/to/pack.json bash tools/opengrep/install-opengrep.sh .
EOF
  exit 1
fi

echo "installing opengrep gate into $REPO_NAME (branch $BRANCH, runner label $LABEL)"
echo "source pack: $FULL_PACK"

install -d tools/opengrep
for f in gate.sh gen-rules.py capture-pack.sh README.md; do
  [ -f "$SRC/$f" ] || continue
  # Skip when the source IS the destination (the installer lives inside the repo
  # it is installing into), which would otherwise abort with "same file".
  [ "$SRC/$f" -ef "tools/opengrep/$f" ] && continue
  cp "$SRC/$f" "tools/opengrep/$f"
done
[ -f "$SRC/gate.sh" ] && [ ! "$SRC/gate.sh" -ef tools/opengrep-gate.sh ] && cp "$SRC/gate.sh" tools/opengrep-gate.sh
chmod +x tools/opengrep-gate.sh tools/opengrep/gen-rules.py tools/opengrep/capture-pack.sh

# Trim to this repo's languages.
python3 tools/opengrep/gen-rules.py "$FULL_PACK" tools/opengrep/rules.yml . | sed 's/^/  /'

# Standalone workflow. No `paths:` filter on purpose: a security scan that only
# runs when certain files change is a scan that silently misses the rest.
cat > .github/workflows/security.yml <<YAML
name: Security

# Static analysis in its own workflow rather than a step in an existing job.
# Repos here split CI across path-filtered workflows, which would run the scan
# several times on one push and not at all on another.
on:
  push:
    branches: [$BRANCH, $OTHER]
  pull_request:
    branches: [$BRANCH, $OTHER]
  workflow_dispatch:

permissions:
  contents: read

concurrency:
  group: \${{ github.workflow }}-\${{ github.ref }}
  cancel-in-progress: true

jobs:
  static-analysis:
    name: static analysis
    runs-on: [self-hosted, linux, $LABEL]
    steps:
      - uses: actions/checkout@v4

      # The runner does not read ~/.bashrc, so the pinned install is added to
      # PATH explicitly. See docs/ci-static-analysis.md for why /opt and not a
      # package manager: this must be the same version on every host.
      - name: opengrep
        run: |
          # Prefer the PINNED install. A distro/AUR package is a different build
          # that can change without a commit, which makes findings unreproducible.
          if [ -x /opt/opengrep/bin/opengrep ]; then
            export PATH="/opt/opengrep/bin:\$PATH"
          elif [ -x \$HOME/.local/bin/opengrep ]; then
            export PATH="\$HOME/.local/bin:\$PATH"
          else
            echo "opengrep not installed at /opt/opengrep/bin or ~/.local/bin" >&2
            echo "install it: sudo bash tools/opengrep/install-opengrep.sh" >&2
            exit 3
          fi
          opengrep --version
          bash tools/opengrep-gate.sh
YAML

# Makefile target, when there is a Makefile to extend.
if [ -f Makefile ] && ! grep -q '^opengrep:' Makefile; then
  printf '\n## opengrep: static analysis; ERROR-severity findings fail, warnings are reported\nopengrep:\n\t@bash tools/opengrep-gate.sh\n' >> Makefile
  echo "  added 'make opengrep' target"
fi

echo
echo "done. Files:"
echo "  tools/opengrep-gate.sh        the gate (two-tier exit, coverage caveat)"
echo "  tools/opengrep/rules.yml      vendored rules, trimmed to this repo"
echo "  tools/opengrep/gen-rules.py   regenerate after a pack bump"
echo "  .github/workflows/security.yml"
echo
echo "Next: prove it can fail before trusting it --"
echo "  inject a finding, run 'make opengrep', confirm exit 1, revert."
