package query

import "testing"

// Time-aware comparison: RFC3339 strings and epoch numbers compare
// chronologically; everything else keeps its existing behavior.
func TestCompareTime(t *testing.T) {
	cases := []struct {
		name string
		a    any
		op   string
		b    any
		want bool
	}{
		// RFC3339 strings
		{"after", "2024-03-23T09:00:00.000Z", ">", "2024-01-01T00:00:00.000Z", true},
		{"before", "2024-01-01T00:00:00.000Z", "<", "2024-03-23T09:00:00.000Z", true},
		{"equal-instant-diff-serial", "2024-01-01T00:00:00Z", "=", "2024-01-01T00:00:00.000Z", true},
		{"not-after", "2024-01-01T00:00:00.000Z", ">", "2024-03-23T09:00:00.000Z", false},
		{"neq-instants", "2024-01-01T00:00:00.000Z", "!=", "2024-03-23T09:00:00.000Z", true},
		// epoch numbers: seconds and millis
		{"epoch-sec-after", 1711184400, ">", 1704067200, true},
		{"epoch-millis-after", 1711184400000, ">", 1704067200000, true},
		// mixed: epoch number vs RFC3339 string (2024-01-01T00:00:00Z == 1704067200)
		{"mixed-sec", 1704067200, "=", "2024-01-01T00:00:00.000Z", true},
		{"mixed-millis", 1711184400000, ">", "2024-01-01T00:00:00.000Z", true},
		// non-date strings never become times
		{"plain-string-gt", "b", ">", "a", false},
		{"plain-string-eq", "evil.example.com", "=", "evil.example.com", true},
		{"plain-string-neq", "a", "!=", "b", true},
		// numeric strings stay numeric, never dates (confidence "85" is a number)
		{"numeric-string", "85", ">", 60, true},
		{"nondate-string-vs-number", "abc", ">", 60, false},
		// existing numeric behavior preserved
		{"numbers", 5, ">", 3, true},
		{"numbers-eq", 5, "=", 5, true},
		{"bool-false", true, ">", false, false},
	}
	for _, tc := range cases {
		if got := compare(tc.a, tc.op, tc.b); got != tc.want {
			t.Errorf("%s: compare(%v, %q, %v) = %v, want %v", tc.name, tc.a, tc.op, tc.b, got, tc.want)
		}
	}
}

func TestToTimeUnitRule(t *testing.T) {
	// abs >= 1e11 is millis, else seconds; verified against known instants.
	sec, ok := toTime(1704067200) // 2024-01-01T00:00:00Z
	if !ok || sec.Year() != 2024 || sec.Month() != 1 || sec.Day() != 1 {
		t.Fatalf("seconds interpreted wrong: %v ok=%v", sec, ok)
	}
	ms, ok := toTime(1711184400000) // 2024-03-23T09:00:00Z
	if !ok || ms.Year() != 2024 || ms.Month() != 3 || ms.Day() != 23 {
		t.Fatalf("millis interpreted wrong: %v ok=%v", ms, ok)
	}
	if _, ok := toTime("not a date"); ok {
		t.Fatal("plain string must not parse as time")
	}
	if _, ok := toTime("85"); ok {
		t.Fatal("numeric string must not parse as time")
	}
}
