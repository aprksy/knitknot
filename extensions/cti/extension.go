// ext: cti-ext — CTI extension registration (importer + relationship verbs).
package cti

import (
	"github.com/aprksy/knitknot/extensions"
	"github.com/aprksy/knitknot/pkg/ports/types"
)

// Extension wires the threat-intelligence domain into the registry. // ext: cti-ext
type Extension struct{} // ext: cti-ext

// New returns the threat-intelligence extension. // ext: cti-ext
func New() *Extension { return &Extension{} } // ext: cti-ext

var _ extension.Extension = (*Extension)(nil) // ext: cti-ext

// Name identifies the extension. // ext: cti-ext
func (e *Extension) Name() string { return "threat-intelligence" } // ext: cti-ext

// ctiVerbs pairs each supported edge kind with its target label and match property. // ext: cti-ext
var ctiVerbs = []struct { // ext: cti-ext
	kind    string // ext: cti-ext
	target  string // ext: cti-ext
	matchOn string // ext: cti-ext
}{ // ext: cti-ext
	{EdgeUses, "", "name"},                              // ext: cti-ext — multi-target: malware, attack-pattern, tool, … (empty = match any label)
	{EdgeTargets, LabelVulnerability, "name"},           // ext: cti-ext: targets points at vulnerabilities, not identities
	{EdgeIndicates, LabelMalware, "name"},               // ext: cti-ext
	{EdgeAttributedTo, "", "name"},                      // ext: cti-ext — multi-target: intrusion-set, identity (empty = match any label)
	{EdgeCommunicatesWith, LabelInfrastructure, "name"}, // ext: cti-ext
	{EdgeBasedOn, LabelObservable, "name"},              // ext: cti-ext
	{EdgeDerivedFrom, LabelObservable, "name"},          // ext: cti-ext
	{EdgeObjectRef, LabelObservable, "name"},            // ext: cti-ext
	{EdgeHasTactic, LabelTactic, PropShortname},         // ext: cti-tactics — Has('has-tactic', '<shortname>') resolves on x_mitre_shortname.
} // ext: cti-ext

// Register adds the stix-2.1 importer and exporter plus the CTI relationship verbs. // ext: cti-ext
func (e *Extension) Register(r extension.Registry) error { // ext: cti-ext
	if err := r.RegisterImporter("stix-2.1", NewSTIXImporter()); err != nil { // ext: cti-ext
		return err // ext: cti-ext
	} // ext: cti-ext
	if err := r.RegisterExporter("stix-2.1", NewSTIXExporter()); err != nil { // ext: cti-export
		return err // ext: cti-export
	} // ext: cti-export
	for _, v := range ctiVerbs { // ext: cti-ext
		if err := r.RegisterVerb(v.kind, types.Verb{TargetLabel: v.target, MatchOn: v.matchOn}); err != nil { // ext: cti-ext
			return err // ext: cti-ext
		} // ext: cti-ext
	} // ext: cti-ext
	return nil // ext: cti-ext
} // ext: cti-ext
