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