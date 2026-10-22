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

Issues are the queue of record. An issue may be created by a human or an AI
agent; what matters is that plan and code stay traceable in both directions.

### The traceability marker (required)

Any agent or human who **plans work, opens a PR, or makes a commit** must state
its issue provenance in the PR body (and commit body when one exists), using
this marker, exactly once:

    Issue: <number> | <relationship>

`<relationship>` is one of:

- `refs` — the issue is related context but this PR does not move it forward.
- `advances` — this PR does part of the issue's work; the issue stays open.
- `closes` — merging this PR completes the issue (GitHub auto-closes it; if the
  PR targets a non-default branch, the closer must close the issue manually).
- `none` — genuinely no issue exists and none is needed (typo fixes, CI
  chunk-size bumps, this governance file itself).

Examples:

    Issue: #37 | closes
    Issue: none

An agent that plans work **without an existing issue** and the work is more
than a one-liner must open an issue first, then reference it. An agent that
observes work done in a commit that closes an issue (direct-to-main fixes,
agent patches) must close or comment on the issue, saying which commit did it.

### Issue lifecycle

- **Triage is periodic.** Stale issues get a dated reality-check comment: still
  relevant (with current context) or rotten (close with reason). An issue with
  no comment in 6 months is presumed rotten.
- **An issue that is done but unmerged** is closed when its work lands on the
  integration branch, not when the PR merges.
- **Closing a rotten issue is not losing history** — the closing comment states
  why, and the superseding artifact (ADR, docs, another issue) is linked.

### Where provenance lives on disk

Plans and agent briefs that will produce PRs must carry the same marker in the
plan file (e.g. a `docs/plans/*.md` or an ADR task list), so a governance agent
can diff plan vs. issues without reading chat history:

    <!-- Issue: #37 | advances -->

The rule of thumb: **if a human or agent would benefit from knowing why this
work exists, that reason belongs in an issue, and the work references it.**

### The work loop (how a task flows, with drift guards)

Every task — human idea or agent assignment — flows through the same loop. The
steps are ordered; skipping one is how drift starts.

1. **Plan.** Write down what will change and why, in one paragraph or a plan
   file. If the repo already has an issue for it, the plan references it.
2. **Issue wording.** Turn the plan into an issue: title = the change in one
   line, body = motivation, scope (what is explicitly OUT), and the acceptance
   check (how "done" will be verified). An AI agent may draft the wording, but
   the **governance agent files it** and returns the issue number.
3. **Register before working.** Post the issue (or reference an existing one)
   and get its number **before** any branch is created. No issue, no branch.
4. **Branch.** One concern per branch, named after the issue where one exists:
   `issue-<n>-<concern>` (e.g. `issue-38-csv-import`), or the conventional
   `feat/* fix/* chore/* docs/*` prefixes when no issue applies. Never commit to
   the integration branch directly; if you notice you are on the wrong branch,
   stop, stash/switch, and re-anchor before continuing.
5. **Work.** Small commits, conventional prefixes, `Issue: #N | advances` in the
   commit body when the issue exists.
6. **Prove.** `make build && make test` (and `make lint`) — real output, real
   machine name, pasted into the PR body.
7. **Pull again before the PR.** Re-fetch the integration branch and the issue
   state right before opening the PR: the issue may have been closed, split, or
   superseded while you worked; your marker must reflect the fresh state.
8. **PR with the marker.** Exactly one `Issue: #N | <relationship>` line in the
   body. The relationship is chosen against the freshly pulled issue state.
9. **Close the loop.** On merge (or landing on the integration branch), the
   issue is closed if the marker said `closes`. An agent that did step 2-8 for a
   human reports the issue number and PR link back, so the human's plan file and
   GitHub agree.

### Drift guards (what agents must check, in humans' favor)

Agents are the consistency muscle. When working with (or for) a human, an agent
must check — and politely refuse to violate — these:

- **No issue, no branch.** If a human asks for work that needs a branch but has
  no issue, the agent drafts the issue wording, files it via the governance
  agent, and waits for the number before creating the branch.
- **Wrong-branch detection.** Before the first commit, verify the current branch
  is a work branch, not the integration branch. If the human is mid-work on the
  integration branch, surface it and propose the fix (new branch + cherry-pick
  or stash), not silent compliance.
- **Stale-issue detection.** Before referencing an issue, re-read its current
  state (docs/issues.md snapshot or GitHub). An issue closed by someone else in
  the meantime is not a valid `closes` target.
- **Marker integrity.** Every PR gets exactly one marker. `none` is a claim to
  verify, not a default: if the machine cannot check GitHub, write
  `Issue: unknown-needs-classification` and let the governance agent classify it.
- **One concern.** A PR that mixes a fix with an unrelated cleanup gets split,
  even when the human asked for "just do both quickly".

The point of the loop is not bureaucracy: it is that a stranger (or a governance
agent) can reconstruct **why** any change exists from the repo alone — plan,
issue, branch, commits, PR — without chat history.

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
