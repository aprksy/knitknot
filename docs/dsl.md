# KnitKnot DSL Reference

The KnitKnot DSL (Domain-Specific Language) allows expressive graph queries using a fluent syntax.

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