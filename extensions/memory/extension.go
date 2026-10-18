package memory

import (
	"github.com/aprksy/knitknot/extensions"
	"github.com/aprksy/knitknot/pkg/ports/types"
)

// Extension wires the memory domain into the registry.
type Extension struct{}

// New returns the memory extension.
func New() *Extension { return &Extension{} }

var _ extension.Extension = (*Extension)(nil)

// Name identifies the extension.
func (e *Extension) Name() string { return "memory" }

// memoryVerbs pairs each supported edge kind with its target label and match property.
var memoryVerbs = []struct {
	kind    string
	target  string
	matchOn string
}{
	{EdgeChildOf, "", PropMemoryID},
	{EdgeSimilarTo, "", PropMemoryID},
	{EdgeSupersedes, "", PropMemoryID},
	{EdgeTagged, LabelTag, PropName},
}

// Register adds the memory-bundle importer plus the memory relationship verbs.
func (e *Extension) Register(r extension.Registry) error {
	if err := r.RegisterImporter("memory-bundle", NewMemoryImporter()); err != nil {
		return err
	}
	for _, v := range memoryVerbs {
		if err := r.RegisterVerb(v.kind, types.Verb{TargetLabel: v.target, MatchOn: v.matchOn}); err != nil {
			return err
		}
	}
	return nil
}
