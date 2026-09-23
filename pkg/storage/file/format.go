package file

import (
	"github.com/aprksy/knitknot/pkg/ports/types"
)

// SavedGraph represents serialized state
type SavedGraph struct {
	Version string                 `json:"version"`
	Nodes   map[string]*types.Node `json:"nodes"`
	Edges   map[string]*types.Edge `json:"edges"`
	Verbs   map[string]types.Verb  `json:"verbs"`
	// version: persist — event log + rev counter (v0.3). Absent (zero)
	// in legacy files; Load synthesizes history for those nodes.
	RevCounter uint64        `json:"revCounter"`
	EventLog   []types.Event `json:"eventLog"`
}

const CurrentVersion = "knitknot/v0.3" // version: persist — adds History/DeletedAt/CreatedRev on Node + top-level event log
