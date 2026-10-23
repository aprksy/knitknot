# GOVERNANCE.md

Operating rules for this repository. **Read this before making changes** — it is
written for both humans and AI agents, and the agent section is binding.

---

## Machines

Repositories are cloned on several machines. They are named, not numbered, and
the name is the one to use in commits, notes, issue comments and this file.

| Name | Address | Access | Role |
|---|---|---|---|
| `sxi-work` | `10.0.10.2` | `ssh sxi-work` (port 2022) | **Primary work machine.** 12 cores, biggest disk. Work starts and is verified here |
| `oci-srv` | `10.0.10.7` | `ssh oci-srv` | **Second machine.** Used when `sxi-work` is unreachable. Follows — pull only, never leads |
| `main-vnic` | `10.0.10.1` | local | WireGuard hub, LLM gateway (`:8080`), Hermes node. **Infra only — no clones** |
| `phone` | `10.0.10.5` | `ssh phone` | Field node (proot Ubuntu in Termux) |

- Refer to a machine by name, not IP: "verified on `sxi-work`", not "10.0.10.2".
- **`main-vnic` is not a workspace.** It holds no working clones; do not start
  work there.
- `sxi-work` is where work happens. `oci-srv` is a follower: keeping it in sync
  is a `git pull`, and it is *expected* to be behind.

---

## Branching

```
main        protected-by-convention; always buildable; merge only via PR
dev         integration branch; feature work lands here first
feat/<topic>  fix/<topic>  chore/<topic>  docs/<topic>   short-lived, one concern
```

- **Never commit directly to `main`.** Open a PR from `dev` (or a `feat/*`
  branch) into `main`.
- Long-lived branch is **`dev`** — not `development`, not `develop`. One name.
- Branch names are lower-case, hyphenated, no dots (`feat/query-time`, not
  `dev.cmd`). Dots read as file extensions to tooling.
- Delete a `feat/*` branch once its PR merges.

> **Note:** `main` is not yet protected by GitHub rules, so the rules above are
> convention, not enforcement. Treat them as binding anyway; the protection
> lands later.

## Commits

Conventional Commits, imperative mood, subject ≤ 72 chars:

```
feat(query): time-aware Where comparisons (RFC3339 + epoch)
fix(sync): warn when the source backend carries no verbs
ci: GitHub Actions (build + vet + gofmt + go test)
```

Prefixes: `feat` `fix` `chore` `docs` `refactor` `test` `perf` `build` `ci`.

Explain **why** in the body when the change is not self-evident. A commit that
only says what changed is half a commit.

## Pull requests

- One concern per PR. If you cannot describe it in one sentence, split it.
- The PR body must state: **what changed**, and **how it was verified** (the
  actual command run, and its result — not "should work").
- `dev` → `main` PRs are reviewed and merged by the repository owner.
- Mention in the PR body if the change touches a file another machine also
  edits (see *Multi-host* below).

## Issues & provenance

Issues are the queue of record, filed directly by whoever does the work
(human or AI agent - every coding machine has `gh` authenticated).

### The rule (required)

1. **Issue before branch.** Work that is more than a one-liner gets an issue
   first; the branch is cut after the issue exists and is named
   `issue-<n>-<concern>` (or conventional `feat/* fix/* chore/* docs/*` when
   no issue applies).
2. **The traceability marker.** Every PR body carries exactly one line:

       Issue: <number> | <relationship>

   `<relationship>`: `refs` (related context), `advances` (partial work,
   issue stays open), `closes` (merging completes the issue - GitHub
   auto-closes on the default branch), `none` (genuinely no issue needed).
3. **Human merge.** PRs are reviewed and merged by the repository owner.
   The coding agent never merges its own work.
4. **Ideation docs** (plans, strategies, ADRs) are plain `docs/` PRs - no
   staging branch, no relay.

### Issue lifecycle

- Stale issues get a dated reality-check comment: still relevant (with
  current context) or rotten (closed with reason + link to the superseding
  artifact). No comment in 6 months = presumed rotten.
- An issue is closed when its work lands on the integration branch.
- Closing a rotten issue states why; nothing is silently deleted.

### Drift guards

- **No issue, no branch** - open the issue first.
- **Wrong-branch check** before the first commit; if you are on an
  integration branch, stop and move.
- **Marker integrity**: `none` is a claim, not a default. A PR without a
  marker gets bounced by the reviewer.
- **One concern per PR**, even when asked to "just do both quickly".

The rule of thumb: if a human or agent would benefit from knowing why this
work exists, that reason belongs in an issue, and the work references it.

## Makefile contract

Reserved command names, identical meaning in every repository, so an agent can
run them without reading the Makefile first.

**Bare form is the aggregate; suffixed form is scoped.**

| Reserved | Meaning | Scoped forms in this repo |
|---|---|---|
| `make build` | build **everything** | — |
| `make test` | run **all** tests | `make test-memory` (extensions/memory only) |
| `make lint` | lint **everything** | — |
| `make run` | run the CLI | — |
| `make clean` | remove all artefacts | — |

Rules:

- **The bare form must exist and must be the aggregate.** `make test` runs
  `go test ./...` — every package. An agent must be able to run `make test` and
  get *everything*.
- **`make lint` must not hard-depend on a tool that may be absent.** It uses
  `go vet` + a `gofmt` check, both of which come with Go, so it is meaningful on
  every machine and in CI.
- **If a reserved name genuinely does not apply**, define it as a no-op that
  prints why. Do not omit it, and do not repurpose it for something else.

### Generated files

Build output is generated, never committed. The `.slim/` directory is a tool's
checkout, not source — any command that scans the tree must skip it.

`extensions/cti/testdata/enterprise-attack.json` (52MB) is **not committed** —
it is gitignored. Regenerate or fetch on demand if CTI tests need it.

## Public API surface

This repository is consumed as a Go **module** by other projects (axiomflow).
What follows is therefore a compatibility contract, not a style preference.

**Importable by other projects:**

| Package | Role |
|---|---|
| `pkg/ports/types` | Core types: `Node`, `Edge`, `Snapshot`, `Event`, `Subgraph`, `Verb`, `VerbRegistry` |
| `pkg/ports/storage` | `Backend` interface, `Selector` — register storage backends by scheme |
| `pkg/ports/query` | `QueryEngine`, `ResultSet`, `Filter`, `Direction`, `PatternNode`, `PatternEdge` |
| `pkg/graph` | `GraphEngine` — kernel primitives: `GetNodeAt`, `GetNodeHistory`, lineage, bounded traversal |
| `pkg/query` | `DefaultQueryEngine` — `Find`, `Has`, `Where` (incl. time-aware `>`/`<`), `Follow`, `Reach` (bounded), `Limit` |
| `pkg/dsl` | Query builder: `Query`, `Node`, `Expression`, string/number literals |
| `pkg/storage/bolt` | `bolt.Storage` — crash-safe single-file backend (`Open`, `OpenNoSync`, `OpenTemp`) |
| `pkg/storage/inmem` | `inmem.Storage` — ephemeral backend (`New`) |
| `pkg/exporter/parquet` | `ExportToParquet` — versioned export for the out-of-band indexer |
| `extensions` | `Extension`, `Importer`, `Exporter`, `Registry`, `NewRegistry` — domain plugin contract |
| `extensions/memory` | Vocabulary consts (`LabelUserProfile`, `EdgeChildOf`, `PropMemoryID`, …), bundle types, `MemoryImporter` — the memory domain layer (on the `omos/memory-contract` lane branch, merged into `main`) |

**Not importable — internal or executable-only:** `cmd/`, `extensions/cti`,
`extensions/dfir`, `.slim/`. These may change at any time.

Rules that follow from being a dependency:

- **Exported names in the importable set are public API.** Renaming or
  changing the signature of an exported symbol is a breaking change and needs a
  major version bump, per semver. Adding is fine; removing is not.
- **Do not move a package into the public set without review.** The moment
  another project imports it, its whole surface is frozen.
- **`extensions/memory` is the entry point for axiomflow's memory subsystem.**
  Consumers should import `extensions/memory` and the port packages, not
  internals, so internal restructuring stays off the compatibility surface.

**Versioning.** Releases are git tags of the form `vMAJOR.MINOR.PATCH` on
`main`, following semver. There is no release branch — a consumer pins a tag:

```
require github.com/aprksy/knitknot v0.1.0
```

Anything published as `v0.x` may still change; `v1.0.0` declares the public
surface stable.

## Definition of done

- `make build` and `make test` pass locally
- `make lint` is clean
- No secrets, tokens, keys, or `.env` files staged or committed
- New behaviour has a test
- The PR body records how it was verified

---

## For AI agents

**These rules are binding. They exist because you are not the only writer.**

1. **Never push to `main`.** Not directly, not with `--force`. Open a PR.
2. **Never force-push a shared branch** (`main`, `dev`). Force-push only your
   own short-lived `feat/*` branch, and only before its PR is opened.
3. **Run `make build && make test` before opening a PR.** Paste the real output
   into the PR body. If it fails, say so — a red PR with an honest note is worth
   more than a green claim nobody verified.
4. **One concern per PR.** Do not bundle an unrelated cleanup into a fix.
5. **Never commit secrets.** Not in code, not in tests, not in fixtures, not in
   commit messages. If you find a committed secret, report it — do not quietly
   delete it, and do not rotate anything yourself.
6. **Do not rewrite history on shared branches.** No rebase, no amend, no
   squash of already-pushed commits.
7. **Report what you could not do.** An unfinished task stated plainly beats a
   plausible-sounding claim. Never fabricate command output.
8. **Name the machine you worked from** when it matters — in PR bodies and
   issue comments. "Verified on `sxi-work`" is actionable; "verified locally"
   is not.
9. **Respect the `.gitignore` anchors.** Patterns are anchored deliberately;
   see *Ignore rules* below before relaxing one.
10. **This repo never calls an LLM, never computes an embedding, never runs
    similarity math.** If you are tempted to add any of these, stop and read
    `docs/domains/memory/README.md` — that boundary is the contract with
    axiomflow. Embeddings, LLM reasoning, and vector indexing are injected
    from outside.

### Ignore rules — the common trap

A bare `vendor/` in `.gitignore` matches **every** directory named `vendor` at
any depth. An unanchored pattern that looks scoped is not. Before adding a broad
pattern, check it:

```bash
git check-ignore -v path/to/file    # shows WHICH rule matched
```

Two traps have already cost this project time:

- **Required-but-ignored files** are the most common cause of "works locally,
  fails in CI". If CI fails on a clean checkout with a missing file, suspect
  this first.
- **`.slim/worktrees/` is a tool's checkout, not source.** It contains full
  copies of this repo. Anything that scans the tree (`gofmt -l .`, a linter, a
  file count) must exclude it or it reports the same problem twice.

### Before you claim something works

Run it. A build that compiled is not a test that passed, and a test that passed
on your machine is not a clean checkout. Locally-present-but-ignored files make
failures invisible — CI is the arbiter.

Watch specifically for the **false green**: a command that exits 0 without doing
the work. If a check cannot fail, it is not a check.

---

## Multi-host

Several machines hold clones of this repository. Git is the only coordination
point; there is no shared filesystem.

- **Push before you switch machines.** An unpushed local branch is invisible
  to every other machine.
- **Pull before you start work**, so you are not building on a stale tree.
- If two machines must work concurrently, use **git worktrees** rather than a
  second clone in the same directory.
- The repository of record is GitHub. A clone that has diverged locally can be
  discarded safely once its work is pushed.

## CI

GitHub Actions, on hosted runners. One job: `test` (build + vet + gofmt + go
test).

A self-hosted runner on `sxi-work` is available by adding its label. **A
self-hosted job queues while that machine is offline — it does not fall back to
hosted.** Prefer hosted unless you specifically need that machine.

Self-hosted runners execute arbitrary workflow code as the local user. Never
enable one for a public repository — **this repository is public**, so do not
add a self-hosted runner lane.
