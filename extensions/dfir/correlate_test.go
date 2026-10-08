package dfir

import (
	"context"
	"strings"
	"testing"

	"github.com/aprksy/knitknot/extensions"
	"github.com/aprksy/knitknot/extensions/cti"
	"github.com/aprksy/knitknot/pkg/ports/types"
	"github.com/aprksy/knitknot/pkg/storage/inmem"
)

// correlateBundle is a self-contained CTI graph: a domain indicator and a
// hash indicator both indicating one malware, with the full
// campaign/actor/TTP chain attached.
const correlateBundle = `{
  "type": "bundle",
  "id": "bundle--aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
  "spec_version": "2.1",
  "objects": [
    {"type": "indicator", "spec_version": "2.1",
     "id": "indicator--11111111-1111-1111-1111-111111111111",
     "created": "2024-01-01T00:00:00.000Z", "modified": "2024-01-01T00:00:00.000Z",
     "name": "Tutorial C2", "pattern": "[domain-name:value = 'tutorial-c2.example.com']",
     "pattern_type": "stix", "valid_from": "2024-01-01T00:00:00.000Z"},
    {"type": "indicator", "spec_version": "2.1",
     "id": "indicator--22222222-2222-2222-2222-222222222222",
     "created": "2024-01-01T00:00:00.000Z", "modified": "2024-01-01T00:00:00.000Z",
     "name": "Tutorial Hash", "pattern": "[file:hashes.'SHA-256' = 'a1b2c3d4e5f60718293a4b5c6d7e8f90a1b2c3d4e5f60718293a4b5c6d7e8f90']",
     "pattern_type": "stix", "valid_from": "2024-01-01T00:00:00.000Z"},
    {"type": "malware", "spec_version": "2.1",
     "id": "malware--33333333-3333-3333-3333-333333333333",
     "created": "2024-01-01T00:00:00.000Z", "modified": "2024-01-01T00:00:00.000Z",
     "name": "TutorialBanker", "malware_types": ["banking-trojan"], "is_family": false},
    {"type": "attack-pattern", "spec_version": "2.1",
     "id": "attack-pattern--44444444-4444-4444-4444-444444444444",
     "created": "2024-01-01T00:00:00.000Z", "modified": "2024-01-01T00:00:00.000Z",
     "name": "Credential Dumping"},
    {"type": "campaign", "spec_version": "2.1",
     "id": "campaign--55555555-5555-5555-5555-555555555555",
     "created": "2024-01-01T00:00:00.000Z", "modified": "2024-01-01T00:00:00.000Z",
     "name": "Tutorial Campaign X"},
    {"type": "intrusion-set", "spec_version": "2.1",
     "id": "intrusion-set--66666666-6666-6666-6666-666666666666",
     "created": "2024-01-01T00:00:00.000Z", "modified": "2024-01-01T00:00:00.000Z",
     "name": "Tutorial APT"},
    {"type": "relationship", "spec_version": "2.1",
     "id": "relationship--77777777-7777-7777-7777-777777777777",
     "created": "2024-01-02T00:00:00.000Z", "modified": "2024-01-02T00:00:00.000Z",
     "relationship_type": "indicates",
     "source_ref": "indicator--11111111-1111-1111-1111-111111111111",
     "target_ref": "malware--33333333-3333-3333-3333-333333333333"},
    {"type": "relationship", "spec_version": "2.1",
     "id": "relationship--88888888-8888-8888-8888-888888888888",
     "created": "2024-01-02T00:00:00.000Z", "modified": "2024-01-02T00:00:00.000Z",
     "relationship_type": "indicates",
     "source_ref": "indicator--22222222-2222-2222-2222-222222222222",
     "target_ref": "malware--33333333-3333-3333-3333-333333333333"},
    {"type": "relationship", "spec_version": "2.1",
     "id": "relationship--99999999-9999-9999-9999-999999999999",
     "created": "2024-01-02T00:00:00.000Z", "modified": "2024-01-02T00:00:00.000Z",
     "relationship_type": "uses",
     "source_ref": "malware--33333333-3333-3333-3333-333333333333",
     "target_ref": "attack-pattern--44444444-4444-4444-4444-444444444444"},
    {"type": "relationship", "spec_version": "2.1",
     "id": "relationship--aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
     "created": "2024-01-02T00:00:00.000Z", "modified": "2024-01-02T00:00:00.000Z",
     "relationship_type": "uses",
     "source_ref": "campaign--55555555-5555-5555-5555-555555555555",
     "target_ref": "malware--33333333-3333-3333-3333-333333333333"},
    {"type": "relationship", "spec_version": "2.1",
     "id": "relationship--bbbbbbbb-cccc-dddd-eeee-ffffffffffff",
     "created": "2024-01-02T00:00:00.000Z", "modified": "2024-01-02T00:00:00.000Z",
     "relationship_type": "attributed-to",
     "source_ref": "campaign--55555555-5555-5555-5555-555555555555",
     "target_ref": "intrusion-set--66666666-6666-6666-6666-666666666666"}
  ]
}`

func seedCorrelateGraph(t *testing.T) *inmem.Storage {
	t.Helper()
	s := inmem.New()
	if err := cti.NewSTIXImporter().Import(context.Background(), extension.ImportContext{}, s, types.NewVerbRegistry(), strings.NewReader(correlateBundle)); err != nil {
		t.Fatalf("import: %v", err)
	}
	return s
}

func refNames(refs []NodeRef) []string {
	names := make([]string, len(refs))
	for i, r := range refs {
		names[i] = r.Name
	}
	return names
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestCorrelateChain(t *testing.T) {
	s := seedCorrelateGraph(t)
	caseObs := []CaseObservable{
		{Observable: Observable{Type: "domain", Value: "tutorial-c2.example.com"}, Context: "proxy log"},
		{Observable: Observable{Type: "file-hash-sha256", Value: "a1b2c3d4e5f60718293a4b5c6d7e8f90a1b2c3d4e5f60718293a4b5c6d7e8f90"}},
		{Observable: Observable{Type: "ipv4", Value: "203.0.113.9"}},
	}
	matches := Correlate(s, caseObs)
	if len(matches) != 2 {
		t.Fatalf("matches = %d, want 2 (domain + hash; ipv4 misses)", len(matches))
	}

	dom := matches[0]
	if dom.Observable.Type != "domain" || dom.Context != "proxy log" {
		t.Errorf("match[0] = %+v, want domain with context carried", dom.Observable)
	}
	if got := refNames(dom.Indicators); !equalStrings(got, []string{"Tutorial C2"}) {
		t.Errorf("indicators = %v", got)
	}
	if got := refNames(dom.Malware); !equalStrings(got, []string{"TutorialBanker"}) {
		t.Errorf("malware = %v", got)
	}
	if got := refNames(dom.Campaigns); !equalStrings(got, []string{"Tutorial Campaign X"}) {
		t.Errorf("campaigns = %v", got)
	}
	if got := refNames(dom.Actors); !equalStrings(got, []string{"Tutorial APT"}) {
		t.Errorf("actors = %v", got)
	}
	if got := refNames(dom.TTPs); !equalStrings(got, []string{"Credential Dumping"}) {
		t.Errorf("ttps = %v", got)
	}
	for _, r := range append(append(append(dom.Indicators, dom.Malware...), dom.Campaigns...), dom.Actors...) {
		if r.ID == "" || r.Label == "" || r.StixID == "" {
			t.Errorf("ref missing identity fields: %+v", r)
		}
	}

	hash := matches[1]
	if hash.Observable.Type != "file-hash-sha256" {
		t.Errorf("match[1] type = %q, want file-hash-sha256", hash.Observable.Type)
	}
	if got := refNames(hash.Indicators); !equalStrings(got, []string{"Tutorial Hash"}) {
		t.Errorf("hash indicators = %v", got)
	}
	if got := refNames(hash.Malware); !equalStrings(got, []string{"TutorialBanker"}) {
		t.Errorf("hash malware = %v", got)
	}
}

func TestCorrelateMissYieldsNone(t *testing.T) {
	s := seedCorrelateGraph(t)
	matches := Correlate(s, []CaseObservable{
		{Observable: Observable{Type: "domain", Value: "nope.example.com"}},
	})
	if len(matches) != 0 {
		t.Fatalf("matches = %d, want 0", len(matches))
	}
}

func TestCorrelateCaseInsensitiveValue(t *testing.T) {
	s := seedCorrelateGraph(t)
	// Uppercase domain in the case still matches via canonicalization.
	matches := Correlate(s, []CaseObservable{
		{Observable: Observable{Type: "domain", Value: "Tutorial-C2.EXAMPLE.com."}},
	})
	if len(matches) != 1 {
		t.Fatalf("matches = %d, want 1", len(matches))
	}
}

func TestParseCaseCSV(t *testing.T) {
	t.Run("aliases context blanks header case", func(t *testing.T) {
		in := "TYPE,VALUE,Context\n" +
			"\n" +
			"SHA-256,ABCDEF,hash hit\n" +
			"ip, 1.2.3.4 ,\n" +
			"File-Name,evil.exe\n"
		got, err := ParseCaseCSV(strings.NewReader(in))
		if err != nil {
			t.Fatalf("ParseCaseCSV: %v", err)
		}
		want := []CaseObservable{
			{Observable: Observable{Type: "file-hash-sha256", Value: "ABCDEF"}, Context: "hash hit"},
			{Observable: Observable{Type: "ipv4", Value: "1.2.3.4"}},
			{Observable: Observable{Type: "file-name", Value: "evil.exe"}},
		}
		if len(got) != len(want) {
			t.Fatalf("got %d rows %v, want %d", len(got), got, len(want))
		}
		for i := range got {
			if got[i] != want[i] {
				t.Errorf("row %d = %+v, want %+v", i, got[i], want[i])
			}
		}
	})

	t.Run("more aliases", func(t *testing.T) {
		in := "type,value\nsha1,a\ndomain-name,b\nipv4-addr,c\nipv6,d\nurl,e\nfilename,f\nmd5,g\ndomain,h\n"
		got, err := ParseCaseCSV(strings.NewReader(in))
		if err != nil {
			t.Fatalf("ParseCaseCSV: %v", err)
		}
		wantTypes := []string{"file-hash-sha1", "domain", "ipv4", "ipv6", "url", "file-name", "file-hash-md5", "domain"}
		for i, wt := range wantTypes {
			if got[i].Type != wt {
				t.Errorf("row %d type = %q, want %q", i, got[i].Type, wt)
			}
		}
	})

	t.Run("unknown type errors", func(t *testing.T) {
		_, err := ParseCaseCSV(strings.NewReader("type,value\nemail,a@b.test\n"))
		if err == nil || !strings.Contains(err.Error(), "row 2") || !strings.Contains(err.Error(), "email") {
			t.Errorf("err = %v, want row-and-type naming error", err)
		}
	})

	t.Run("missing value errors", func(t *testing.T) {
		_, err := ParseCaseCSV(strings.NewReader("type,value\ndomain,\n"))
		if err == nil || !strings.Contains(err.Error(), "row 2") {
			t.Errorf("err = %v, want row 2 missing value error", err)
		}
	})

	t.Run("missing type errors", func(t *testing.T) {
		_, err := ParseCaseCSV(strings.NewReader("type,value\n,example.com\n"))
		if err == nil {
			t.Error("err = nil, want missing type error")
		}
	})
}
