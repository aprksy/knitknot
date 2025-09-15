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
}

const CurrentVersion = "knitknot/v0.1"
