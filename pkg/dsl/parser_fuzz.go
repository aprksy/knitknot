//go:build !exclude_from_coverage

package dsl

import (
	"testing"
)

// FuzzParser ensures the DSL parser never panics on random input.
func FuzzParser(f *testing.F) {
	// Seed with known valid queries
	f.Add("Find('User')")
	f.Add("Has('has_skill', 'Go')")
	f.Add("Where('n.age', '>', 30)")
	f.Add("Find('User').Has('has_skill', 'Go')")
	f.Add("Find('X').Where('a.b', '=', 1).Limit(5)")

	// Seed with malformed but common patterns
	f.Add("Find(User)")
	f.Add("'")
	f.Add("((")
	f.Add(".Has(x,y)")
	f.Add("Find()..Has()")

	f.Fuzz(func(t *testing.T, input string) {
		parser := NewParser(input)
		_, _ = parser.Parse()
		// No need to assert correctness — just don't crash
	})
}
