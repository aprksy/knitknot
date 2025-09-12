package query

import (
	"github.com/aprksy/knitknot/pkg/ports/query"
	"github.com/aprksy/knitknot/pkg/ports/types"
)

var _ query.ResultSet = (*ResultSet)(nil)

// ResultSet holds the results of a query execution.
// Each item is a mapping from variable name (e.g., "n", "s") to Node.
type ResultSet struct {
	items []map[string]*types.Node
}

// Len returns the number of rows.
func (rs *ResultSet) Len() int {
	return len(rs.items)
}

// Empty checks if no results were found.
func (rs *ResultSet) Empty() bool {
	return len(rs.items) == 0
}

// Empty checks if no results were found.
func (rs *ResultSet) Items() []map[string]*types.Node {
	return rs.items
}
