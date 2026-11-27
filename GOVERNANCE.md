# GOVERNANCE.md

Operating rules for this repository. **Read this before making changes** — it is
written for both humans and AI agents, and the agent section is binding.

---

## Machines

Repositories are cloned on several machines. They are named, not numbered, and
the name is the one to use in commits, notes, issue comments and this file.

| Name | Address | Access | Role |
|---|---|---|---|
| `oci-srv` | `10.0.10.7` | `ssh oci-srv` | **Default dev host** (aarch64, 2 vCPU). Code, light CI, light serving, small test runs. Also production — do not saturate |
| `sxi-work` | `10.0.10.2` | `ssh sxi-work` (port 2022) | **Heavy lane** (x86_64, 12 cores, biggest disk). Heavy CI, mobile/godot/playwright/client builds. Work queues here rather than moving to `oci-srv` |
| `main-vnic` | `10.0.10.1` | local | WireGuard hub, LLM gateway (`:8080`), Hermes node. **Infra only — no clones** |
| `phone` | `10.0.10.5` | `ssh phone` | Field node (proot Ubuntu in Termux) |

- Refer to a machine by name, not IP: "verified on `sxi-work`", not "10.0.10.2".
- **`main-vnic` is not a workspace.** It holds no working clones; do not start
  work there.
- Work starts on `oci-srv`. Heavy or device-bound work **waits for `sxi-work`**; it does not move to `oci-srv`. Refer to a machine by name, not IP.

---

## Spec-driven milestones

Work is specified before it is built, and a milestone — not a feature — is the unit of governance.

1. **Constitution** (`.specify/memory/constitution.md`) — one page, invariants only: what this repo
   must never break. It restates the rules in this file; it does not describe the project. Amend it by
   PR, never inline.
2. **One spec per milestone** (`specs/<nnn>-<slug>/spec.md`) — problem, scope, the invariants
   it touches, and acceptance criteria a test can fail. One page. Forward-looking only: existing
   behaviour is referenced (`STATUS.md`, `contracts/`), never re-described.
3. **Plan, then tasks** — `/speckit.plan` then `/speckit.tasks`. The task list becomes the issue list:
   one issue per task, each carrying its acceptance line.
4. **A milestone closes when** every task issue is closed, its acceptance criteria are demonstrated in
   the PR that closes them, and the spec is updated to say so.

`.specify/memory/current-state.md` is an agent-written half page — entry points, build/test commands,
invariants found, known debt — that `/speckit.specify` reads instead of guessing. It is reviewed by
exception, not line by line.

## Worktrees

Worktrees live **inside the project directory**, never loose in `$HOME`:

- `.slim/worktrees/<lane>` — lanes owned by the `slim` tooling
- `.worktree/<name>` — ad-hoc workspace for a task or an agent

Both paths are git-ignored. A worktree at `~/<project>-<task>` or `~/homelabs/<task>` is a review
blocker: it hides the branch from the project's `git worktree list` and outlives the PR that created it.

- Remove the worktree in the same pass that merges its branch. A merged branch does not need a checkout.
- Never leave uncommitted work in a worktree. Commit it to the branch first — the worktree is
  disposable, the commit is not.
- `/tmp/<project-name>` (e.g. `/tmp/clichse-probe`) is always allowed for throwaway verification
  clones. Nothing there is authoritative and it may be deleted at any time.

## Branching

```
main        protected-by-convention; always buildable; merge only via PR
dev         integration branch; feature work lands here first
feat/<topic>  fix/<topic>  chore/<topic>  docs/<topic>   short-lived, one concern
```

- **Never commit directly to `main`.** Feature PRs target `dev`; `main` moves
  only by a promotion PR from `dev`.
- **`dev` contains every commit on `main`.** When a commit reaches `main`
  without `dev`, reconcile `main` back into `dev` instead of letting the two
  drift.
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

       Issue: #<number> | <relationship>

   `<relationship>`: `refs` (related context), `advances` (partial work,
   issue stays open), `closes` (merging completes the issue - GitHub
   auto-closes on the default branch), `none` (genuinely no issue needed).
   The `#` is load-bearing: GitHub only links and auto-closes on the
   `closes #N` form. `Issue: 63 | closes` matches nothing — that exact
   mistake shipped twice (exporter PRs) before this line was tightened.
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

## Agent handoff

**This section is identical in every repository of the fleet.** It exists because
work moves between agents — **Hermes**, **opencode** (and any agent added later) —
and between machines, often mid-task. Read it before you touch anything; update it
before you hand off.

### 1. Declare who you are

State your agent name in the first line of every PR body, issue comment and
handoff note:

```
Agent: hermes            # or: opencode
Host:  ocisrv            # the machine you actually ran on
```

Never sign work with a name you did not run under. "Verified on `sxi-work`" is
actionable; "verified locally" is not.

### 2. Never leave work invisible

Work an agent cannot see does not exist. Before you stop:

- **Push** every branch you created. An unpushed branch is invisible to the next
  agent and to every other machine.
- **Open a PR** for anything that is not a throwaway experiment, even if it is
  incomplete — mark it `Draft:` and say what is missing.
- **Record blocked work** as a `blocked` issue, or as a task in the PR body. A
  half-finished lane with no issue is the most expensive thing you can leave
  behind.
- **Never commit to `main`.** `dev` is the integration branch; feature work lands
  there first.

### 3. The four questions every handoff must answer

Write these in the PR body, or in `.specify/memory/current-state.md` when the work
is not PR-shaped yet:

1. **What changed** — files and behaviour, not intent.
2. **How it was verified** — the actual command and its real result. Never
   "should work". Never invented output.
3. **What is still pending** — the next concrete step, and who or what is
   blocked on it.
4. **What you could not do** — stated plainly. An unfinished task named honestly
   is worth more than a green claim nobody checked.

### 4. Current state (verified snapshot)

Refresh this table whenever you change a repo's state, and at minimum in the same
PR that changes it. A stale snapshot is worse than none — it sends the next agent
down a path that no longer exists.

```
repo             branch                        dirty  ahead  last commit
aptimetric       issue-32-item-manifest        1      0      5c12191
axiomflow        chore/bump-knitknot-v0.4.0    0      0      6d87e07
clichse          dev                           0      0      78c6e721
knitgen          chore/knitknot-v040-sync      0      0      b7b0edd
knitknot         (detached HEAD)               0      n/a    6876c45
kronika          chore/bump-knitknot-v0.4.0    0      0      08149c6
labkit           dev                           0      0      52b9565
llm-gateway      main                          0      0      64247a0
naughty-kittens  dev                           0      0      8b6de39
petricore        dev                           0      0      085b7bf
siem-lite        dev                           0      2      4090e9a
ultramech        dev                           41     0      43175d5
```

Re-measure rather than trusting the table:

```bash
for d in ~/homelabs/dev/*/; do [ -d "$d/.git" ] || continue
  cd "$d"; printf "%-17s %-30s dirty=%-3s ahead=%-4s %s\n" \
    "$(basename "$d")" "$(git branch --show-current)" \
    "$(git status --porcelain | wc -l)" \
    "$(git rev-list --count @{u}..HEAD 2>/dev/null || echo n/a)" \
    "$(git log -1 --format=%h)"
done
```

`ahead=n/a` means the branch has no upstream — it was never pushed.

### 5. Pending work

Only what is genuinely in flight. Delete rows as they land; never leave a row
behind that has already shipped.

- **siem-lite** — `dev` is 2 commits ahead of `origin/dev`, unpushed. Push before
  anyone else touches it.
- **ultramech** — 41 untracked files, all Godot `.uid` sidecars. Decide once
  whether `.uid` files are committed or ignored, then apply it fleet-wide.
- **aptimetric** — `uv.lock` modified and uncommitted on `issue-32-item-manifest`.
- **knitknot** — detached HEAD at `6876c45`. Attach to a branch before working.

### 6. Switching agents mid-task

- **Read `git status`, `git log -1` and the open PRs before your first edit.** The
  previous agent may have left work you cannot see in a branch.
- **Do not re-do finished work.** Check the PR list and the snapshot above first.
- **Do not force-push or rebase a shared branch** (`main`, `dev`) to tidy up
  someone else's commits.
- **Continue the same branch and PR** rather than opening a parallel one for the
  same concern. Two PRs on one concern is a merge conflict with extra steps.
- **Commit identity is the repo's, not the agent's.** Never change `user.name` or
  `user.email`; attribution goes in the PR body, not the author field.

### 7. Machine access when you take over

Clones live on `ocisrv` and `sxi-work`. `ocimain` is the WireGuard hub and holds
no clones. If you cannot reach a host, say so instead of working around it — a
silent fallback to a different machine produces work verified somewhere the owner
did not expect.

### 8. Definition of done, for a handoff

A task is handoff-ready when all of these hold:

- `make check` passes, and its real output is in the PR body.
- The branch is pushed and a PR is open (or a `blocked` issue is filed).
- Section 3's four questions are answered.
- Section 4's snapshot and Section 5's pending list are updated.
- Nothing is left uncommitted that the next agent would need.

<!-- fleet-handoff:v1 -->
