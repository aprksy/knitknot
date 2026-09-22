package cti_test

import (
	"testing"

	"github.com/aprksy/knitknot/extensions/cti"
)

func TestParseSTIXIDValid(t *testing.T) {
	typ, uuid, err := cti.ParseSTIXID("indicator--8e2e2d2b-17d4-4cbf-938f-11f0b995a000")
	if err != nil {
		t.Fatalf("ParseSTIXID valid: unexpected error: %v", err)
	}
	if typ != "indicator" {
		t.Errorf("type = %q, want %q", typ, "indicator")
	}
	if uuid != "8e2e2d2b-17d4-4cbf-938f-11f0b995a000" {
		t.Errorf("uuid = %q, unexpected", uuid)
	}
}

func TestParseSTIXIDErrors(t *testing.T) {
	cases := map[string]string{
		"indicator8e2e2d2b-17d4-4cbf-938f-11f0b995a000":   "missing separator",
		"--8e2e2d2b-17d4-4cbf-938f-11f0b995a000":          "empty type",
		"Indicator--8e2e2d2b-17d4-4cbf-938f-11f0b995a000": "uppercase type",
		"indicator--not-a-uuid":                           "malformed UUID",
		"indicator--8E2E2D2B-17D4-4CBF-938F-11F0B995A000": "uppercase UUID",
		"indicator--": "empty UUID",
		"":            "empty input",
	}
	for id, why := range cases {
		if _, _, err := cti.ParseSTIXID(id); err == nil {
			t.Errorf("ParseSTIXID(%q) (%s): expected error, got nil", id, why)
		}
	}
}

func TestCanonicalizeDomain(t *testing.T) {
	if got := cti.CanonicalizeDomain("Example.COM."); got != "example.com" {
		t.Errorf("CanonicalizeDomain(Example.COM.) = %q, want example.com", got)
	}
	if got := cti.CanonicalizeDomain("example.com"); got != "example.com" {
		t.Errorf("CanonicalizeDomain(example.com) = %q, want unchanged", got)
	}
	if got := cti.CanonicalizeDomain(""); got != "" {
		t.Errorf("CanonicalizeDomain(\"\") = %q, want empty", got)
	}
}

func TestCanonicalizeHash(t *testing.T) {
	got, err := cti.CanonicalizeHash("SHA-256", "ABCDEF0123456789")
	if err != nil {
		t.Fatalf("CanonicalizeHash: unexpected error: %v", err)
	}
	if got != "abcdef0123456789" {
		t.Errorf("CanonicalizeHash = %q, want lowercase digest", got)
	}
	for _, tc := range [][2]string{
		{"SHA-256", ""},
		{"", "abcdef"},
		{"SHA-256", "xyz-not-hex"},
	} {
		if _, err := cti.CanonicalizeHash(tc[0], tc[1]); err == nil {
			t.Errorf("CanonicalizeHash(%q, %q): expected error, got nil", tc[0], tc[1])
		}
	}
}

func TestCanonicalizeCVE(t *testing.T) {
	got, err := cti.CanonicalizeCVE("cve-2021-44228")
	if err != nil {
		t.Fatalf("CanonicalizeCVE: unexpected error: %v", err)
	}
	if got != "CVE-2021-44228" {
		t.Errorf("CanonicalizeCVE = %q, want CVE-2021-44228", got)
	}
	if _, err := cti.CanonicalizeCVE("garbage"); err == nil {
		t.Error("CanonicalizeCVE(garbage): expected error, got nil")
	}
}

func TestCanonicalizeURL(t *testing.T) {
	got, err := cti.CanonicalizeURL("HTTPS://Example.COM/Some/Path?Q=1")
	if err != nil {
		t.Fatalf("CanonicalizeURL: unexpected error: %v", err)
	}
	if got != "https://example.com/Some/Path?Q=1" {
		t.Errorf("CanonicalizeURL = %q, want scheme+host lowered, path preserved", got)
	}
	if _, err := cti.CanonicalizeURL("   "); err == nil {
		t.Error("CanonicalizeURL(empty): expected error, got nil")
	}
}
