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
                | WhereMethod
                | WhereEdgeMethod
                | LimitMethod
                | InMethod ;
HasMethod   = ".Has(" String "," String ")" ;
FollowMethod = ".Follow(" String [ "," Direction ] ")" ;
FollowHasMethod = ".FollowHas(" String "," String [ "," Direction ] ")" ;
WhereMethod = ".Where(" String "," String "," Value ")" ;
WhereEdgeMethod = ".WhereEdge(" String "," String "," Value ")" ;
LimitMethod = ".Limit(" Number ")" ;
InMethod    = ".In(" String ")" ;

String      = "'" <any char except '> "'".
Number      = digit+
Value       = String | Number
Direction   = "'out'" | "'in'" | "'both'"
```

## Example
TODO