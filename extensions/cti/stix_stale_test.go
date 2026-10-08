package cti

import "testing"

// Pure unit tests for the staleness guard: no parser involved, so missing
// and malformed timestamps (which stix2 would reject at parse time) can be
// covered directly. Marking-definitions, for example, carry no `modified`.
func TestIsStale(t *testing.T) {
	props := func(modified any, has bool) map[string]any {
		m := map[string]any{}
		if has {
			m["modified"] = modified
		}
		return m
	}
	cases := []struct {
		name string
		old  map[string]any
		new  map[string]any
		want bool
	}{
		{"older-new", props("2024-02-01T00:00:00.000Z", true), props("2024-01-01T00:00:00.000Z", true), true},
		{"newer-new", props("2024-01-01T00:00:00.000Z", true), props("2024-02-01T00:00:00.000Z", true), false},
		{"equal", props("2024-01-01T00:00:00.000Z", true), props("2024-01-01T00:00:00.000Z", true), false},
		{"missing-old", props(nil, false), props("2024-01-01T00:00:00.000Z", true), false},
		{"missing-new", props("2024-01-01T00:00:00.000Z", true), props(nil, false), false},
		{"garbage-old", props("bogus", true), props("2024-01-01T00:00:00.000Z", true), false},
		{"garbage-new", props("2024-01-01T00:00:00.000Z", true), props("bogus", true), false},
		{"non-string", props(12345, true), props("2024-01-01T00:00:00.000Z", true), false},
		{"offset-format", props("2024-02-01T00:00:00+00:00", true), props("2024-01-01T00:00:00.000Z", true), true},
	}
	for _, tc := range cases {
		if got := isStale(tc.old, tc.new); got != tc.want {
			t.Errorf("%s: isStale=%v, want %v", tc.name, got, tc.want)
		}
	}
}
