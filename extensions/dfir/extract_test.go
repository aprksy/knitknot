package dfir

import "testing"

func TestExtractObservables(t *testing.T) {
	sha := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	tests := []struct {
		name    string
		pattern string
		want    []Observable
		wantOK  bool
	}{
		{
			name:    "sha256 single quotes",
			pattern: "[file:hashes.'SHA-256' = '" + sha + "']",
			want:    []Observable{{Type: "file-hash-sha256", Value: sha}},
			wantOK:  true,
		},
		{
			name:    "sha256 double-quoted algo",
			pattern: `[file:hashes."SHA-256" = 'abc']`,
			want:    []Observable{{Type: "file-hash-sha256", Value: "abc"}},
			wantOK:  true,
		},
		{
			name:    "sha256 bare algo",
			pattern: `[file:hashes.SHA-256 = 'abc']`,
			want:    []Observable{{Type: "file-hash-sha256", Value: "abc"}},
			wantOK:  true,
		},
		{
			name:    "sha256 extra whitespace double-quoted value",
			pattern: `[  file:hashes.'SHA-256'   =   "abc"  ]`,
			want:    []Observable{{Type: "file-hash-sha256", Value: "abc"}},
			wantOK:  true,
		},
		{
			name:    "sha1",
			pattern: "[file:hashes.'SHA-1' = 'abc123']",
			want:    []Observable{{Type: "file-hash-sha1", Value: "abc123"}},
			wantOK:  true,
		},
		{
			name:    "md5",
			pattern: "[file:hashes.'MD5' = 'd41d8cd98f00b204e9800998ecf8427e']",
			want:    []Observable{{Type: "file-hash-md5", Value: "d41d8cd98f00b204e9800998ecf8427e"}},
			wantOK:  true,
		},
		{
			name:    "file name",
			pattern: "[file:name = 'evil.exe']",
			want:    []Observable{{Type: "file-name", Value: "evil.exe"}},
			wantOK:  true,
		},
		{
			name:    "domain",
			pattern: "[domain-name:value = 'example.com']",
			want:    []Observable{{Type: "domain", Value: "example.com"}},
			wantOK:  true,
		},
		{
			name:    "ipv4",
			pattern: "[ipv4-addr:value = '1.2.3.4']",
			want:    []Observable{{Type: "ipv4", Value: "1.2.3.4"}},
			wantOK:  true,
		},
		{
			name:    "ipv6",
			pattern: "[ipv6-addr:value = '::1']",
			want:    []Observable{{Type: "ipv6", Value: "::1"}},
			wantOK:  true,
		},
		{
			name:    "url with = in value",
			pattern: "[url:value = 'https://example.com/x?a=b']",
			want:    []Observable{{Type: "url", Value: "https://example.com/x?a=b"}},
			wantOK:  true,
		},
		{
			name:    "OR yields multiple in order",
			pattern: "[file:hashes.'SHA-256' = 'aaa'] OR [domain-name:value = 'example.com']",
			want: []Observable{
				{Type: "file-hash-sha256", Value: "aaa"},
				{Type: "domain", Value: "example.com"},
			},
			wantOK: true,
		},
		{
			name:    "OR lowercase with extra spaces",
			pattern: "  [ipv4-addr:value = '1.2.3.4']   or   [url:value = 'https://x.test/']  ",
			want: []Observable{
				{Type: "ipv4", Value: "1.2.3.4"},
				{Type: "url", Value: "https://x.test/"},
			},
			wantOK: true,
		},
		{
			name:    "or inside quoted value does not split",
			pattern: "[url:value = 'https://or.example.com/']",
			want:    []Observable{{Type: "url", Value: "https://or.example.com/"}},
			wantOK:  true,
		},
		{
			name:    "OR inside brackets extracts both",
			pattern: "[file:name = 'a' OR file:name = 'b']",
			want: []Observable{
				{Type: "file-name", Value: "a"},
				{Type: "file-name", Value: "b"},
			},
			wantOK: true,
		},
		{
			name:    "AND inside brackets extracts supported",
			pattern: "[file:name = 'a' AND file:size = '1']",
			want:    []Observable{{Type: "file-name", Value: "a"}},
			wantOK:  true,
		},
		{
			name:    "two clauses joined by AND extracts both",
			pattern: "[file:name = 'a'] AND [domain-name:value = 'example.com']",
			want: []Observable{
				{Type: "file-name", Value: "a"},
				{Type: "domain", Value: "example.com"},
			},
			wantOK: true,
		},
		{
			name:    "MATCHES operator unsupported",
			pattern: "[domain-name:value MATCHES 'example\\.com']",
			want:    nil,
			wantOK:  false,
		},
		{
			name:    "LIKE operator unsupported",
			pattern: "[file:name LIKE 'evil%']",
			want:    nil,
			wantOK:  false,
		},
		{
			name:    "not-equals unsupported",
			pattern: "[file:name != 'a']",
			want:    nil,
			wantOK:  false,
		},
		{
			name:    "unknown object path unsupported",
			pattern: "[email-address:value = 'a@b.test']",
			want:    nil,
			wantOK:  false,
		},
		{
			name:    "unknown hash algo unsupported",
			pattern: "[file:hashes.'SHA-512' = 'abc']",
			want:    nil,
			wantOK:  false,
		},
		{
			name:    "partial OR returns supported clause",
			pattern: "[domain-name:value = 'example.com'] OR [email-address:value = 'a@b.test']",
			want:    []Observable{{Type: "domain", Value: "example.com"}},
			wantOK:  true,
		},
		{
			name:    "empty string",
			pattern: "",
			want:    nil,
			wantOK:  false,
		},
		{
			name:    "whitespace only",
			pattern: "   ",
			want:    nil,
			wantOK:  false,
		},
		{
			name:    "truncated bracket still extracts comparison",
			pattern: "[domain-name:value = 'example.com'",
			want:    []Observable{{Type: "domain", Value: "example.com"}},
			wantOK:  true,
		},
		{
			name:    "no equals",
			pattern: "[domain-name:value 'example.com']",
			want:    nil,
			wantOK:  false,
		},
		{
			name:    "unterminated quote",
			pattern: "[domain-name:value = 'example.com]",
			want:    nil,
			wantOK:  false,
		},
		{
			name:    "garbage",
			pattern: "not a pattern",
			want:    nil,
			wantOK:  false,
		},
		{
			name:    "unbalanced bracket no panic",
			pattern: "[[[[",
			want:    nil,
			wantOK:  false,
		},
		{
			name:    "AND with unquoted size constraint extracts hash",
			pattern: "[file:hashes.'SHA-256' = 'a'] AND [file:size = 100]",
			want:    []Observable{{Type: "file-hash-sha256", Value: "a"}},
			wantOK:  true,
		},
		{
			name:    "WITHIN qualifier does not suppress",
			pattern: "[domain-name:value = 'evil.com'] WITHIN 300 SECONDS",
			want:    []Observable{{Type: "domain", Value: "evil.com"}},
			wantOK:  true,
		},
		{
			name:    "mixed OR AND extracts supported pair",
			pattern: "[file:name = 'x'] OR [file:hashes.'SHA-256' = 'y'] AND [process:name = 'z']",
			want: []Observable{
				{Type: "file-name", Value: "x"},
				{Type: "file-hash-sha256", Value: "y"},
			},
			wantOK: true,
		},
		{
			name:    "AND inside one bracket extracts hash",
			pattern: "[file:hashes.'SHA-256' = 'a' AND file:size = 100]",
			want:    []Observable{{Type: "file-hash-sha256", Value: "a"}},
			wantOK:  true,
		},
		{
			name:    "qualifier after OR extracts both",
			pattern: "[domain-name:value = 'a.com'] OR [ipv4-addr:value = '1.2.3.4'] WITHIN 300 SECONDS",
			want: []Observable{
				{Type: "domain", Value: "a.com"},
				{Type: "ipv4", Value: "1.2.3.4"},
			},
			wantOK: true,
		},
		{
			name:    "FOLLOWEDBY extracts both sides",
			pattern: "[file:name = 'a'] FOLLOWEDBY [domain-name:value = 'b'] WITHIN 60 SECONDS",
			want: []Observable{
				{Type: "file-name", Value: "a"},
				{Type: "domain", Value: "b"},
			},
			wantOK: true,
		},
		{
			name:    "START STOP qualifiers do not suppress",
			pattern: "[ipv4-addr:value = '1.2.3.4'] START t'2024-01-01T00:00:00Z' STOP t'2024-01-02T00:00:00Z'",
			want:    []Observable{{Type: "ipv4", Value: "1.2.3.4"}},
			wantOK:  true,
		},
		{
			name:    "lowercase and",
			pattern: "[file:name = 'a'] and [domain-name:value = 'b']",
			want: []Observable{
				{Type: "file-name", Value: "a"},
				{Type: "domain", Value: "b"},
			},
			wantOK: true,
		},
		{
			name:    "three-way OR",
			pattern: "[file:hashes.'SHA-256' = 'a'] OR [domain-name:value = 'b'] OR [ipv4-addr:value = 'c']",
			want: []Observable{
				{Type: "file-hash-sha256", Value: "a"},
				{Type: "domain", Value: "b"},
				{Type: "ipv4", Value: "c"},
			},
			wantOK: true,
		},
		{
			name:    "paren-wrapped clause",
			pattern: "([ipv4-addr:value = '1.2.3.4'])",
			want:    []Observable{{Type: "ipv4", Value: "1.2.3.4"}},
			wantOK:  true,
		},
		{
			name:    "url with query string",
			pattern: "[url:value = 'https://evil.example.com/a?b=1&c=2']",
			want:    []Observable{{Type: "url", Value: "https://evil.example.com/a?b=1&c=2"}},
			wantOK:  true,
		},
		{
			name:    "url with space in path",
			pattern: "[url:value = 'http://a.com/path with space']",
			want:    []Observable{{Type: "url", Value: "http://a.com/path with space"}},
			wantOK:  true,
		},
		{
			name:    "md5 bare algo",
			pattern: "[file:hashes.MD5 = 'abc123']",
			want:    []Observable{{Type: "file-hash-md5", Value: "abc123"}},
			wantOK:  true,
		},
		{
			name:    "sha1 double-quoted algo and value",
			pattern: `[file:hashes."SHA-1" = "aabbcc"]`,
			want:    []Observable{{Type: "file-hash-sha1", Value: "aabbcc"}},
			wantOK:  true,
		},
		{
			name:    "unquoted size constraint skipped",
			pattern: "[file:size = 12345]",
			want:    nil,
			wantOK:  false,
		},
		{
			name:    "dst_ref path unsupported",
			pattern: "[network-traffic:dst_ref.value = '1.2.3.4']",
			want:    nil,
			wantOK:  false,
		},
		{
			name:    "process name unsupported even beside supported",
			pattern: "[process:name = 'cmd.exe']",
			want:    nil,
			wantOK:  false,
		},
		{
			name:    "LIKE on supported path not extracted",
			pattern: "[file:hashes.'SHA-256' LIKE 'abc%']",
			want:    nil,
			wantOK:  false,
		},
		{
			name:    "hash value keeps raw form for canonicalization",
			pattern: "[file:hashes.'SHA-256' = 'AbC123  ']",
			want:    []Observable{{Type: "file-hash-sha256", Value: "AbC123  "}},
			wantOK:  true,
		},
		{
			name:    "domain value keeps raw form for canonicalization",
			pattern: "[domain-name:value = 'Evil.COM.']",
			want:    []Observable{{Type: "domain", Value: "Evil.COM."}},
			wantOK:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ExtractObservables(tc.pattern)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v (obs=%v)", ok, tc.wantOK, got)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("got %d observables %v, want %d %v", len(got), got, len(tc.want), tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("obs[%d] = %+v, want %+v", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestCanonical(t *testing.T) {
	tests := []struct {
		name string
		obs  Observable
		want string
	}{
		{"domain lowercase+trailing dot", Observable{"domain", "EXAMPLE.COM."}, "example.com"},
		{"domain keeps single dot only", Observable{"domain", "EXAMPLE.COM.."}, "example.com."},
		{"hash uppercase+spaces", Observable{"file-hash-sha256", " E3B0 C442 "}, "e3b0c442"},
		{"sha1 lowercase+strip ws", Observable{"file-hash-sha1", "\tABC DEF\n"}, "abcdef"},
		{"md5 lowercase+strip ws", Observable{"file-hash-md5", " D41D 8CD9 "}, "d41d8cd9"},
		{"ipv4 trim only", Observable{"ipv4", "  1.2.3.4  "}, "1.2.3.4"},
		{"ipv6 trim only", Observable{"ipv6", "  ::1 "}, "::1"},
		{"url preserves case", Observable{"url", "  HTTPS://Example.COM/Path  "}, "HTTPS://Example.COM/Path"},
		{"file-name preserves case", Observable{"file-name", "  Evil.EXE "}, "Evil.EXE"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.obs.Canonical(); got != tc.want {
				t.Errorf("Canonical() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestKeyDiffersAcrossTypes(t *testing.T) {
	a := Observable{Type: "domain", Value: "example.com"}
	b := Observable{Type: "url", Value: "example.com"}
	if a.Key() == b.Key() {
		t.Errorf("keys collide: %q", a.Key())
	}
	c := Observable{Type: "domain", Value: "EXAMPLE.COM."}
	if a.Key() != c.Key() {
		t.Errorf("canonically equal keys differ: %q vs %q", a.Key(), c.Key())
	}
}
