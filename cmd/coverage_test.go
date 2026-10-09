package cmd

import (
	"strings"
	"testing"

	"github.com/aprksy/knitknot/extensions/dfir"
)

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

func TestFormatOrderedLine(t *testing.T) {
	half := 0.5
	line := formatOrderedLine(dfir.DimensionCoverage{Label: "attack-pattern", OrderedCoverage: &half, OrderedN: 2})
	if line != "ordered 1/2 50%" {
		t.Errorf("line = %q, want ordered 1/2 50%%", line)
	}
	line = formatOrderedLine(dfir.DimensionCoverage{Label: "attack-pattern"})
	if line != "ordered n/a (N=0)" {
		t.Errorf("line = %q, want ordered n/a (N=0)", line)
	}
	third := float64(1) / float64(3)
	line = formatOrderedLine(dfir.DimensionCoverage{Label: "attack-pattern", OrderedCoverage: &third, OrderedN: 3})
	if line != "ordered 1/3 33%" {
		t.Errorf("line = %q, want ordered 1/3 33%%", line)
	}
}

func TestFormatCoverageTextOrdered(t *testing.T) {
	half := 0.5
	rep := dfir.CoverageReport{
		Target: dfir.NodeRef{Label: "intrusion-set", Name: "Ordered APT"},
		Dimensions: []dfir.DimensionCoverage{
			{Label: "indicator", Covered: 1, Total: 1,
				Matched:   []dfir.NodeRef{{Name: "Ordered C2"}},
				Unmatched: []dfir.NodeRef{}},
			{Label: "attack-pattern", Covered: 2, Total: 4,
				Matched:         []dfir.NodeRef{{Name: "Command Shell"}, {Name: "Remote Desktop"}},
				Unmatched:       []dfir.NodeRef{{Name: "Credential Dumping"}},
				OrderedCoverage: &half, OrderedN: 2,
				ActualOrder:   []dfir.NodeRef{{Name: "Remote Desktop"}, {Name: "Command Shell"}},
				ExpectedOrder: []dfir.NodeRef{{Name: "Command Shell"}, {Name: "Remote Desktop"}}},
		},
		Inferred: []dfir.NodeRef{{Name: "Ordered Malware"}},
	}
	out := formatCoverageText(rep)
	for _, want := range []string{
		"ordered 1/2 50%",
		"expected: Command Shell, Remote Desktop",
		"actual: Remote Desktop, Command Shell",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("text lacks %q:\n%s", want, out)
		}
	}
}
