package cti_test

import (
	"testing"

	"github.com/aprksy/knitknot/extensions/cti"
)

func TestSupportedLabelsRoundTrip(t *testing.T) {
	for _, l := range cti.SupportedLabels {
		if !cti.IsSupportedLabel(l) {
			t.Errorf("IsSupportedLabel(%q) = false, want true", l)
		}
	}
	if cti.IsSupportedLabel("not-a-label") {
		t.Error("IsSupportedLabel(not-a-label) = true, want false")
	}
	if cti.IsSupportedLabel("") {
		t.Error("IsSupportedLabel(\"\") = true, want false")
	}
}

func TestSupportedEdgeKindsRoundTrip(t *testing.T) {
	for _, k := range cti.SupportedEdgeKinds {
		if !cti.IsSupportedEdgeKind(k) {
			t.Errorf("IsSupportedEdgeKind(%q) = false, want true", k)
		}
	}
	if cti.IsSupportedEdgeKind("not-an-edge") {
		t.Error("IsSupportedEdgeKind(not-an-edge) = true, want false")
	}
	if cti.IsSupportedEdgeKind("") {
		t.Error("IsSupportedEdgeKind(\"\") = true, want false")
	}
}

func TestLabelAndEdgeKindValues(t *testing.T) {
	if len(cti.SupportedLabels) != 11 {
		t.Errorf("len(SupportedLabels) = %d, want 11", len(cti.SupportedLabels))
	}
	if len(cti.SupportedEdgeKinds) != 8 {
		t.Errorf("len(SupportedEdgeKinds) = %d, want 8", len(cti.SupportedEdgeKinds))
	}
}
