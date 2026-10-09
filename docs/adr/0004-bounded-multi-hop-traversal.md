# ADR 0004 — Bounded multi-hop traversal

- **Status:** proposed
- **Date:** 2026-10-08
- **Deciders:** knitknot maintainers

## Context

The DSL can only express **single-hop** relationships. `Has(rel, value)` and
`Where`-chained queries all originate their edges at the `Find` node, so a
chain like `Find('campaign').Has('uses','X').Has('uses','Y')` fans out from
the campaign (campaigns using both), rather than tracing a path. This is
documented as a limitation in the CTI GUIDE and blocks several domains:

- **CTI:** `actor → campaign → malware → attack-pattern` as one query
  (today: stepwise pivots).
- **DFIR:** attack-chain reconstruction across hops.
- **Firewall/network reachability:** "is host B reachable from A within N
  hops".
- General path-shaped questions.

**Verified before writing this ADR:** the query engine *already* traverses
chained `PatternEdge`s. `DefaultQueryEngine.expandViaEdge` expands from
`edgePattern.From` using the current row, so a plan whose edges chain
`n → v1 → v2` resolves correctly (confirmed with a scratch test:
`A -e1→ B -e2→ C` returns `{n:A, v1:B, v2:C}`). The gaps are purely in the
**builder/DSL wiring** (the builder always sources edges from `n`) and the
absence of a **variable-length** operator.

## Decision

Add bounded multi-hop traversal in two tiers. Both are read-only; neither
mutates storage.

### Naming: verbs are methods, prepositions are arguments

The DSL is verb-method based (`Find`, `Has`, `Where`, `Limit`, `In`) — plain,
short, ops-English, not graph-academic. `Follow` and `Reach` are **actions**
in that register, so they become methods. `Via` and `Within` are
**qualifiers** — "by way of", "within N" — so they are expressed as
argument/option values, not methods: *via* is the edge-kind argument and
*within* is the depth argument. The two naming families are therefore not
rivals; they occupy different roles.

Verb choices by tone (settled): `Follow` for the single-hop chain (sequency;
`then` reads as a logical connective, `hop` as graph jargon), and `Reach` for
bounded reachability (`traverse` is too technical for an ops DSL).

### Tier 1 — fixed-length chains

Allow edges to chain from the previous hop's variable, and expose it in the
DSL. This is the verified engine capability, wired up.

```text
Find('intrusion-set')
  .Follow('attributed-to', In)   // actor ←attributed-to← campaign   (incoming)
  .Follow('uses', Out)           // campaign →uses→ malware
  .Follow('uses', Out)           // malware →uses→ attack-pattern
```

- `.Follow(rel string, dir Direction)` — one hop along `rel` from the **last
  matched variable** (not `n`).
- `.FollowHas(rel, value string, dir Direction)` — `.Follow` plus a target
  `name` filter (the chaining analog of `Has`).
- `Has` keeps its current fan-out semantics (backward compatible).

### Tier 2 — bounded variable-length reachability

```text
Find('host').Where('n.name','=','node1').Reach('', Out)           // default depth
Find('host').Where('n.name','=','node1').Reach('tcp', Out, 5)     // explicit depth
Find('host').Where('n.name','=','node1').Reach('routes', In)
Find('host').Where('n.name','=','node1').Reach('', Out, 9).Where('n.name','=','node-n')
```

- `.Reach(rel string, dir Direction, maxDepth ...int)` — every node reachable
  in **1..maxDepth** hops along edges of kind `rel` (empty = any kind).
  `maxDepth` is optional; omitted uses the default (below).
- Direction is an **argument** (`Out`/`In`/`Both`), not separate methods.
- "Reachable within N steps" (existence) is `Reach(...).Where('n.name','=',…)`
  then a non-empty check — no separate boolean operator.

### Tier 3 — deferred

Constrained paths ("via node2 **or** node4"), all-paths enumeration,
shortest path, and path-returning output are **out of scope** for this ADR.
They are a different cost class (path tracking / exponential enumeration)
and should be a separate decision once Tier 2 shows real demand.

## Plan + engine design

Extend the existing ordered edge list rather than adding a parallel plan
structure, so chained hops stay in one sequence:

```go
type Direction int

const (
    Out  Direction = iota // default
    In
    Both
)

type PatternEdge struct {
    From, To string
    Kind     string
    Filters  []Filter
    Direction Direction // Out when zero
    MinDepth  int       // default 1
    MaxDepth  int       // default 1; >1 => bounded traversal
}
```

- **Tier 1** = several `PatternEdge`s with `MaxDepth == 1`, chained by var.
- **Tier 2** = one `PatternEdge` with `MaxDepth > 1`.

Engine (`DefaultQueryEngine`):

- `expandViaEdge` gains direction awareness: `Out` uses `GetEdgesFrom`, `In`
  uses `GetEdgesTo`, `Both` unions them.
- When `MaxDepth > 1`, run a **BFS from `row[From]`** to depth `MaxDepth`,
  along edges of kind `Kind` (empty = any), honoring `Direction`, and emit
  **one row per reached node** (with `To` set to it) for depths in
  `[MinDepth, MaxDepth]`.
- Reached nodes must satisfy the `To` var's label constraint (existing
  `findLabelForVar` check) before emission.
- Node filters (`Where` on the `To` var) apply afterward via the existing
  `applyAllFilters`.

## Semantics

- **Cycle safety.** The operator returns the **set of reachable nodes**, using
  a per-source visited set. This makes it inherently cycle-safe and O(V+E) —
  it does *not* enumerate paths, so cycles cannot blow up. (Path enumeration
  is Tier 3 and would need caps.)
- **Depth is defaulted and capped.** `.Reach` without an explicit `maxDepth`
  uses `DefaultReachDepth` (proposed: **8** — network-hop idiomatic: "seven
  hops to anywhere"); any explicit or default value is clamped to
  `MaxReachDepth` (proposed: **32**). BFS is O(V+E) regardless of depth
  (visited set), so the bound chiefly governs output size and intent; an
  unbounded/wildcard depth is rejected.
- **Subgraph.** Traversal must respect `plan.Subgraph` (edges/nodes limited to
  membership), consistent with `Execute`'s current subgraph handling.
- **Edge kind filter** is the only edge constraint in Tier 2 (empty = any).
  Edge *property* filters on a traversal are Tier 3.
- **Result size** is bounded by the node set (deduped); implement a hard cap
  on emitted rows as a guard rail.
- **Determinism.** Result order is not currently guaranteed by the engine;
  this ADR does not change that. If stable output is needed, sort at the
  presentation layer.

## Out of scope

- Path enumeration / returning paths
- Constrained paths ("via node X"), shortest path
- Edge-property filters during traversal
- Undirected reachability beyond the `Both` union (no min-cut, no flow)
- Any write/mutation through traversal

## Delivery stages

1. **Tier 1** — `PatternEdge.Direction` + chained builder methods + DSL
   (`.Follow`, `.FollowHas`, direction as a `Direction` argument). Small;
   engine already chains.
2. **Tier 2** — `MinDepth`/`MaxDepth` + BFS in `expandViaEdge` + DSL
   (`.Reach` with direction argument and optional depth) + depth cap + row cap.
3. Optional — Tier 3 path features, gated on demonstrated need.

## Risks

- **Semantics creep** between fixed hops and reachability; keep them distinct
  (`.Follow` = one hop, `.Reach` = bounded set).
- **Cost** if depth/row caps are missing — mitigated by mandatory depth cap
  and node-set (not path) output.
- **Direction bugs** on the CTI chain, whose hops mix directions
  (`attributed-to` is traversed incoming). Tier 1 tests must cover this.
- **Subgraph interaction** — a traversal that ignores `plan.Subgraph` would
  silently cross boundaries; must be tested.

## Consequences

- Closes the multi-hop limitation documented in the CTI GUIDE; enables the
  shared prerequisite for the DFIR and firewall-reachability explorations.
- Backward compatible: `Has` and existing single-hop plans are unchanged
  (`MaxDepth == 0`/`1`).
- Read-only: no storage or kernel invariants change.

## Open decisions

- Whether Tier 2 should also accept edge-property filters in a later revision.
- Whether `.FollowHas` earns its keep, or value filtering on a hop is folded
  into `.Follow` / done with `.Where`.
