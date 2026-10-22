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

	// Seed with write statements (W1: parsed, not executed)
	f.Add("ADDNODE Person name=Alice age=35")
	f.Add("CONNECT n1 --knows since=2020--> n2")
	f.Add("UPDATE NODE n1 age=36")
	f.Add("DELETE EDGE n1 --knows--> n2")
	f.Add("DEFINE has_skill TO Skill VIA name")

	// Seed with malformed but common patterns
	f.Add("Find(User)")
	f.Add("'")
	f.Add("((")
	f.Add(".Has(x,y)")
	f.Add("Find()..Has()")
	f.Add("ADDNODE")
	f.Add("CONNECT n1 --> n2")

	f.Fuzz(func(t *testing.T, input string) {
		parser := NewParser(input)
		_, _ = parser.Parse()
		// ParseProgram shares the lexer and adds the write path — same
		// bar: parse anything without crashing (fresh parser, one-shot).
		_, _ = NewParser(input).ParseProgram()
		// No need to assert correctness — just don't crash
	})
}
