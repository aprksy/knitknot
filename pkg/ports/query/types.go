package query

type PatternNode struct {
	Var   string
	Label string
}

type PatternEdge struct {
	From, To string
	Kind     string
	Filters  []Filter
}

type Filter struct {
	Field string
	Op    string
	Value any
}
