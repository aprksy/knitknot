package query

import (
	"encoding/json"
	"fmt"

	"github.com/aprksy/knitknot/pkg/ports/query"
	"github.com/aprksy/knitknot/pkg/ports/types"
)

var _ query.ResultSet = (*ResultSet)(nil)

func NewResultSet(items []map[string]*types.Node) *ResultSet {
	// Make a shallow copy to prevent mutation
	copied := make([]map[string]*types.Node, len(items))
	for i, row := range items {
		copied[i] = copyMap(row)
	}
	return &ResultSet{items: copied}
}

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

func (r *ResultSet) MarshalJSON() ([]byte, error) {
	out := make([]map[string]*types.Node, len(r.items))
	for i, row := range r.items {
		copied := make(map[string]*types.Node)
		for k, v := range row {
			copied[k] = v
		}
		out[i] = copied
	}
	return json.Marshal(out)
}

func (r *ResultSet) String() string {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Sprintf("ResultSet(error: %v)", err)
	}
	return string(data)
}
