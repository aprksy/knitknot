package query

type PatternNode struct {
	Var   string
	Label string
}

type Direction int

const (
	Out Direction = iota // default / zero value
	In
	Both
)

// Reachability depth bounds: default when omitted, hard cap always.
const (
	DefaultReachDepth = 8
	MaxReachDepth     = 32
)

type PatternEdge struct {
	From, To  string
	Kind      string
	Filters   []Filter
	Direction Direction // Out when zero
	MinDepth  int       // 0 ⇒ 1
	MaxDepth  int       // 0 ⇒ 1 (single hop); >1 ⇒ bounded traversal
}

type Filter struct {
	Field string
	Op    string
	Value any
}
