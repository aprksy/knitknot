package types

// Verb defines the meaning of a relationship type (edge kind)
type Verb struct {
	// TargetLabel is the expected label of the destination node
	TargetLabel string

	// MatchOn is the property key used in .Has(rel, value) filtering
	// e.g., for "has_skill" → MatchOn = "name" → WHERE v.name = 'Go'
	MatchOn string
}

// DefaultMatchProperty is used if MatchOn is empty
const DefaultMatchProperty = "name"
