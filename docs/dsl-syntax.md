# KnitKnot DSL Syntax

## Syntax Diagram
A railroad diagram for the DSL syntax is not yet available. See the
BNF below for the authoritative grammar.

## BNF Syntax
```bnf
Query       = FindMethod { ChainableMethod } "Exec()" ;
FindMethod  = "Find(" String ")" ;
ChainableMethod = HasMethod
                | FollowMethod
                | FollowHasMethod
                | ReachMethod
                | ReachHasMethod
                | WhereMethod
                | WhereEdgeMethod
                | LimitMethod
                | InMethod ;
HasMethod   = ".Has(" String "," String ")" ;
FollowMethod = ".Follow(" String [ "," Direction ] ")" ;
FollowHasMethod = ".FollowHas(" String "," String [ "," Direction ] ")" ;
ReachMethod = ".Reach(" String [ "," Direction [ "," Number ] ] ")" ;
ReachHasMethod = ".ReachHas(" String "," String [ "," Direction [ "," Number ] ] ")" ;
WhereMethod = ".Where(" String "," String "," Value ")" ;
WhereEdgeMethod = ".WhereEdge(" String "," String "," Value ")" ;
LimitMethod = ".Limit(" Number ")" ;
InMethod    = ".In(" String ")" ;

String      = "'" <any char except '> "'".
Number      = digit+
SignedNum   = [ "-" ] digit+
Value       = String | Number
Direction   = "'out'" | "'in'" | "'both'"
```

## Write statements (ADR 0009)

One statement per program — either a query (above) or one write.

```bnf
Program     = WriteStatement | Query ;
WriteStatement = AddNodeStmt | ConnectStmt | UpdateStmt
              | DeleteStmt | DefineStmt ;
AddNodeStmt = "ADDNODE" Label { Prop } ;
ConnectStmt = "CONNECT" ID "--" Rel [ Prop ] "-->" ID ;
UpdateStmt  = "UPDATE" ( "NODE" ID { Prop } | "EDGE" EdgeRef { Prop } ) ;
DeleteStmt  = "DELETE" ( "NODE" ID | "EDGE" EdgeRef ) ;
DefineStmt  = "DEFINE" Word "TO" Label "VIA" PropName ;

EdgeRef     = ID "--" Rel "-->" ID ;
Prop        = PropName "=" PropValue ;
PropValue   = String | SignedNum | Word ;
```

- Keywords (`ADDNODE`, `CONNECT`, `UPDATE`, `DELETE`, `DEFINE`, `NODE`,
  `EDGE`, `TO`, `VIA`) match case-insensitively, like the REPL they came from.
- **Props:** values are a quoted string (`name='John Doe'`), a signed
  number (`age=35`, `score=-5`), or a bare word (`active=true`, kept as
  string). Unquoted multi-word values are rejected — quote them
  (the one deliberate tightening vs. the old REPL parser).
- **Writes return IDs** (the API contract): `ADDNODE` returns the new
  node's id; `CONNECT`/`UPDATE EDGE`/`DELETE EDGE` the edge id in
  storage form `from->to@kind`; `UPDATE NODE`/`DELETE NODE` the node id.
- Same grammar everywhere: REPL commands and
  `GraphEngine.Execute` (`dsl.NewParser(s).ParseProgram()`, then
  `engine.Execute(ctx, prog)`) parse identical statements.

## Example

```text
# write: create two nodes and connect them (prints the IDs)
addnode Person name='Alice Smith' age=35
addnode Skill name=Go
CONNECT n1 --has_skill--> n2

# register verb semantics for Has()
DEFINE has_skill TO Skill VIA name

# read: names of people who have the Go skill and are over 30
Find('Person').Has('has_skill', 'Go').Where('n.age', '>', 30)

# write: update and delete (edge ids are from->to@kind)
UPDATE NODE n1 city='Jakarta'
DELETE EDGE n1 --has_skill--> n2
DELETE NODE n2
```