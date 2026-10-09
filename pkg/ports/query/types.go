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

type PatternEdge struct {
	From, To  string
	Kind      string
	Filters   []Filter
	Direction Direction // Out when zero
}

type Filter struct {
	Field string
	Op    string
	Value any
}
