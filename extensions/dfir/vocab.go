package dfir

// STIX 2.1 vocabulary values this domain needs to correlate a case against
// a CTI graph.
//
// Declared locally because docs/domains/README.md forbids a domain
// extension importing another domain extension. These are standard STIX
// terms, not CTI-private values; the test suite builds its fixtures with
// extensions/cti, so any drift between the two definitions fails tests.
const (
	LabelIndicator  = "indicator"
	LabelObservable = "observable"

	EdgeIndicates    = "indicates"
	EdgeUses         = "uses"
	EdgeAttributedTo = "attributed-to"
	EdgeBasedOn      = "based-on"

	PropStixID     = "stix_id"
	PropSourceFeed = "source_feed"
)
