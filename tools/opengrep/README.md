# opengrep template

Reusable static-analysis gate. Installed per repo; **not** shared at runtime.

## Install into a repo

```bash
cd <repo>
bash tools/opengrep/install-opengrep.sh . <runner-label>
```

The runner label is the repo's own tag (`clichse`, `petricore`, …). The script
writes:

```
tools/opengrep-gate.sh          the gate
tools/opengrep/rules.yml        vendored rules, trimmed to this repo's languages
tools/opengrep/gen-rules.py     regenerate after a pack bump
tools/opengrep/capture-pack.sh  capture a new pack from the registry
.github/workflows/security.yml  standalone workflow
```

plus a `make opengrep` target when a Makefile exists.

## Why a standalone workflow

Repos here split CI across path-filtered workflows — clichse has three
(`backend.yml`, `contracts.yml`, `frontend.yml`). A step added to each would scan
the whole tree repeatedly on some pushes and miss it entirely on others. One
unfiltered workflow covers the tree exactly once, every time.

Consequence: the scan runs even on a docs-only change. That is deliberate — a
security scan with `paths:` filters is a scan that silently skips the commit that
matters.

## The two-tier exit

Bare `opengrep scan` exits **0 regardless of findings** — measured: securai2
produced 97 ERROR-severity findings and exit code 0.

| pass | severity | effect |
|---|---|---|
| gate | `ERROR` | non-zero → red build |
| report | `WARNING` | printed only |

The gate exits **3** (not 1) when opengrep is missing, so "tool not installed" is
distinguishable from "you wrote something bad".

## Rules are vendored

`--config p/default` fetches from `semgrep.dev` on **every run** (measured: 1074
rules fetched, 581 executed). That is a network dependency in CI, a supply-chain
surface, and silent drift — the rule set changes without a commit.

`rules.yml` is pinned and trimmed to the languages the repo actually contains,
derived from tracked files at install time. clichse (TS/TSX + Go) keeps 629 of
1074; llm-gateway (Go) keeps 634 and runs 9.2s → 5.6s.

## Bumping the pack

```bash
bash tools/opengrep/capture-pack.sh /tmp/new-pack.json .
python3 tools/opengrep/gen-rules.py /tmp/new-pack.json tools/opengrep/rules.yml .
```

Reviewing 1.4 MB of JSON is impractical, so review the **rule id list**:

```bash
python3 -c "import json;print('\n'.join(sorted(r['id'] for r in json.load(open('tools/opengrep/rules.yml'))['rules'])))" | diff old.txt new.txt
```

Commit the pack with the id diff in the message.

## Suppressing a finding

Targeted only, on the line immediately above the match, with the **prefixed** id:

```go
// nosemgrep: tools.opengrep.go.lang.security.filepath-clean-misuse.filepath-clean-misuse
name := path.Clean(x)
```

Two silent traps:

- **The id carries a prefix from the rules directory.** With the pack at
  `tools/opengrep/rules.yml`, every id is reported as `tools.opengrep.<id>`. An
  unprefixed `nosemgrep` does nothing.
- **It must be the last comment line before the match.** A prose comment between
  the `nosemgrep` and the flagged line breaks the association.

Prefer fixing. In llm-gateway, `run-shell-injection` was fixed for real by
passing the value through `env:` instead of interpolating `${{ }}` into the
script body.

## Coverage caveat

Files opengrep cannot parse are **not scanned** and do not affect the exit code.
The gate prints them:

```
!! 4 file(s) could not be parsed and were NOT scanned:
```

A green run is not evidence about those files.

## Requirements

opengrep on the runner, pinned, at `/opt/opengrep/bin`:

```bash
sudo bash tools/install-opengrep.sh        # pins the version, no AUR
```

The runner does not read `~/.bashrc`; the workflow adds that path explicitly.
