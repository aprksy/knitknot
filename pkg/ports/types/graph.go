package types

// Node and Edge remain concrete types
type Node struct {
	ID    string
	Label string
	Props map[string]any
}

type Edge struct {
	ID    string
	From  string
	To    string
	Kind  string
	Props map[string]any
}
