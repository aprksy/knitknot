package types

// Node and Edge remain concrete types
type Node struct {
	ID        string         `json:"id"`
	Label     string         `json:"label"`
	Props     map[string]any `json:"props"`
	Subgraphs []string       `json:"subgraphs,omitempty"`
}

type Edge struct {
	ID        string         `json:"id"`
	From      string         `json:"from"`
	To        string         `json:"to"`
	Kind      string         `json:"kind"`
	Props     map[string]any `json:"props"`
	Subgraphs []string       `json:"subgraphs,omitempty"`
}
