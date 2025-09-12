package query

import "github.com/aprksy/knitknot/pkg/ports/types"

// ResultSet holds the results of a query execution.
// Each item is a mapping from variable name (e.g., "n", "s") to Node.
type ResultSet interface {
	Len() int
	Empty() bool
	Items() []map[string]*types.Node
}
