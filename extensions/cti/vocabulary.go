// ext: cti-vocab — STIX 2.1 node labels, edge kinds, and property names.
package cti

// ext: cti-vocab — node labels mirror STIX 2.1 type names verbatim.
const (
	LabelThreatActor    = "threat-actor"
	LabelCampaign       = "campaign"
	LabelMalware        = "malware"
	LabelTool           = "tool"
	LabelVulnerability  = "vulnerability"
	LabelIndicator      = "indicator"
	LabelInfrastructure = "infrastructure"
	LabelIdentity       = "identity"
	LabelReport         = "report"
	LabelObservedData   = "observed-data"
	LabelObservable     = "observable"
)

// ext: cti-vocab — edge kinds map STIX SRO relationship_type values opaquely.
const (
	EdgeUses             = "uses"
	EdgeTargets          = "targets"
	EdgeIndicates        = "indicates"
	EdgeAttributedTo     = "attributed-to"
	EdgeCommunicatesWith = "communicates-with"
	EdgeBasedOn          = "based-on"
	EdgeDerivedFrom      = "derived-from"
	EdgeObjectRef        = "object-ref"
)

// ext: cti-vocab — common STIX property names preserved on import.
const (
	StixID               = "stix_id"
	PropType             = "type"
	PropSpecVersion      = "spec_version"
	PropCreated          = "created"
	PropModified         = "modified"
	PropCreatedByRef     = "created_by_ref"
	PropRevoked          = "revoked"
	PropConfidence       = "confidence"
	PropLang             = "lang"
	PropExternalRefs     = "external_references"
	PropObjectMarkings   = "object_marking_refs"
	PropGranularMarkings = "granular_markings"
	PropLabels           = "labels"
	// ext: cti-vocab — knitknot-side provenance, not STIX-native.
	SourceFeed = "source_feed"
	ImportedAt = "imported_at"
)

// SupportedLabels lists every node label the CTI extension recognizes.
var SupportedLabels = []string{
	LabelThreatActor,
	LabelCampaign,
	LabelMalware,
	LabelTool,
	LabelVulnerability,
	LabelIndicator,
	LabelInfrastructure,
	LabelIdentity,
	LabelReport,
	LabelObservedData,
	LabelObservable,
}

// SupportedEdgeKinds lists every edge kind the CTI extension recognizes.
var SupportedEdgeKinds = []string{
	EdgeUses,
	EdgeTargets,
	EdgeIndicates,
	EdgeAttributedTo,
	EdgeCommunicatesWith,
	EdgeBasedOn,
	EdgeDerivedFrom,
	EdgeObjectRef,
}

var supportedLabelSet = func() map[string]struct{} {
	m := make(map[string]struct{}, len(SupportedLabels))
	for _, l := range SupportedLabels {
		m[l] = struct{}{}
	}
	return m
}()

var supportedEdgeKindSet = func() map[string]struct{} {
	m := make(map[string]struct{}, len(SupportedEdgeKinds))
	for _, k := range SupportedEdgeKinds {
		m[k] = struct{}{}
	}
	return m
}()

// IsSupportedLabel reports whether s is a recognized CTI node label.
func IsSupportedLabel(s string) bool {
	_, ok := supportedLabelSet[s]
	return ok
}

// IsSupportedEdgeKind reports whether s is a recognized CTI edge kind.
func IsSupportedEdgeKind(s string) bool {
	_, ok := supportedEdgeKindSet[s]
	return ok
}
