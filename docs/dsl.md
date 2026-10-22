# KnitKnot DSL Reference

The KnitKnot DSL (Domain-Specific Language) allows expressive graph queries using a fluent syntax.

## Write statements (ADR 0009)

Besides queries, one write statement per program:

```text
ADDNODE Label key=value ...          → creates a node, returns its ID
CONNECT A --rel--> B                 → connects two nodes (props in the arrow: --rel k=v-->)
UPDATE NODE <id> key=value ...       → merges properties into a node
UPDATE EDGE A --rel--> B key=value   → merges properties into an edge
DELETE NODE <id> / DELETE EDGE A --rel--> B
DEFINE <verb> TO <Label> VIA <prop>  → registers relationship semantics
```

- Values are `'quoted strings'`, `numbers` (incl. negative, e.g. `-5`), or bare
  words (`true` stays a string). **Spaced values need quoting:** `name='John Doe'`
  — unquoted `name=John Doe` is rejected.
- Writes return IDs everywhere (REPL output and `GraphEngine.Execute`),
  and every write fails before mutating: a dangling `CONNECT` or a
  missing update/delete target changes nothing.
- `dsl.NewParser(s).ParseProgram()` parses either half (read or write);
  `engine.Execute(ctx, prog)` runs it — the same path the REPL uses.

## Basic Structure
```text
Find(label).Has(rel, value).Where(field, op, val).Limit(n)
```

## Commands 
- `Find(label) `

    Starts a query with nodes of given label. 
    ```
    Find('customer')
    Find('channel')
    ```

- `Has(rel, value) `

    Finds nodes connected by a relationship where the target's property matches value. 
    ```
    # finds channel nodes where .name = 'Marketplace'
    Has('make_purchase_in', 'Marketplace') 

    # finds payment_method where .name = 'Credit Card'
    Has('make_payment_using', 'Credit Card')    
    ```
    Requires verb registration: 
    ```
    DEFINE make_purchase_in TO channel VIA name
    DEFINE make_payment_using TO payment_method VIA name
    ```

- `Follow(rel, dir)` / `FollowHas(rel, value, dir) `

    Chains a hop from the **previous** node instead of the `Find` node, so
    hops form a path. `dir` is `'out'` (default), `'in'`, or `'both'`.
    `FollowHas` also filters the target's `MatchOn` property (like `Has`).
    ```
    # actor <-attributed-to- campaign ->uses- malware ->uses- attack-pattern
    Find('intrusion-set').Follow('attributed-to', 'in').Follow('uses').Follow('uses')

    # chained, with a value filter on the target
    Find('campaign').FollowHas('uses', 'SomeMalware').Follow('uses')
    ```
    Unlike `Has` (which fans out from the `Find` node), `Follow` advances the
    current position along the path. See ADR 0004.

- `Reach(rel, dir, maxDepth)` / `ReachHas(rel, value, dir, maxDepth) `

    Bounded reachability: every node reachable in 1..`maxDepth` hops along
    `rel` edges (empty `''` = any kind), direction-aware. `maxDepth` defaults
    to 8 and is capped at 32. `ReachHas` also filters the reached node's
    `MatchOn` property. The reached node is bound to a **new var** (e.g.
    `v0`); use `ReachHas` (preferred) or `Where` on that var.
    ```
    # nodes 1..2 hops from the actor, either direction
    Find('intrusion-set').Reach('', 'both', 2)

    # existence: is SomeMalware reachable from the actor within 3 steps?
    Find('intrusion-set').ReachHas('', 'SomeMalware', 'both', 3)
    ```
    Unlike `Follow` (one hop), `Reach` explores up to `maxDepth` hops and is
    cycle-safe. See ADR 0004.

- `Where(field, op, value) `

    Filters based on node properties. 
    ```
    Where('n.age', '>', 30)
    Where('v0.level', '=', 5)
    ```
    Field format: {var}.{prop} 
    
    Supported ops: =, !=, >, <
    Comparisons are numeric when both sides coerce to numbers, and
    chronological when both sides resolve to instants (RFC3339 strings, or
    epoch numbers: seconds below 1e11 else millis).

- `WhereEdge(field, value) `

    Filters edges by their properties. 
    ```
    # only edges with trx_amount > 3000
    WhereEdge('trx_amount', '>', 3000)   
    ```

- `Limit(n) `

    Limits results. 
    ```
    Limit(10)
    ```

- `In(subgraph) `

    Restricts query to a subgraph. 
    ```
    In('org')
    ```

## Examples

Find customers who make purchase in marketplace, who is a female, with amount greater than 100 and limit result to 5.
```
Find('customer').Has('make_purchase_in', 'Marketplace').Where('n.age', '>', 25).Where('n.gender', '=', 'female').WhereEdge('trx_amount', '>', '100').Limit(5)
```

Find a customer that lives in Dallas, which has done PayPal transaction more than 5 times.
```
Find('customer').Where('city', '=', 'Dallas').Has('make_payment_using', 'PayPal').WhereEdge('Trx_count', '>', 5)
```

## More on Syntax
- [More technical Ref](dsl-syntax.md)