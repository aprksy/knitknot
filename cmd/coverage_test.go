package cmd

import "testing"

func TestCoveragePct(t *testing.T) {
	if got := coveragePct(1, 2); got != "50%" {
		t.Errorf("coveragePct(1,2) = %q, want 50%%", got)
	}
	if got := coveragePct(0, 0); got != "n/a" {
		t.Errorf("coveragePct(0,0) = %q, want n/a (never 100%%)", got)
	}
	if got := coveragePct(0, 2); got != "0%" {
		t.Errorf("coveragePct(0,2) = %q, want 0%%", got)
	}
}
