package dfir

import "strings"

// Supported STIX 2.1 comparisons (exact-equality, single observable):
//   - [file:hashes.'SHA-256' = 'v'] → file-hash-sha256
//   - [file:hashes.'SHA-1' = 'v']   → file-hash-sha1
//   - [file:hashes.'MD5' = 'v']     → file-hash-md5
//   - [file:name = 'v']             → file-name
//   - [domain-name:value = 'v']     → domain
//   - [ipv4-addr:value = 'v']       → ipv4
//   - [ipv6-addr:value = 'v']       → ipv6
//   - [url:value = 'v']             → url
//
// Tolerated variations: arbitrary whitespace, single/double/bare quotes
// around the hash algorithm name, single/double quotes around the value.
//
// Correlation semantics: every supported "=" comparison in the pattern is
// extracted regardless of the boolean/qualifier structure around it.
// AND/OR/FOLLOWEDBY combinators, grouping parens, brackets, and trailing
// qualifiers (WITHIN n SECONDS, START/STOP timestamps) never suppress a
// supported comparison. Anything else (LIKE/MATCHES/ISSUBSET, comparators
// other than "=", unknown object paths) is skipped per comparison.

// ExtractObservables parses a STIX 2.1 pattern string and returns every
// observable it recognizes. ok=false means the pattern contains no
// supported observable (unsupported or malformed) — callers should count
// it as skipped, never treat it as a match. Never panics on malformed input.
func ExtractObservables(pattern string) (obs []Observable, ok bool) {
	for _, piece := range splitComparisons(pattern) {
		if o, supported := parseComparison(piece); supported {
			obs = append(obs, o)
		}
	}
	if len(obs) == 0 {
		return nil, false
	}
	return obs, true
}

// boolKeywords are the combinators split on; longest first so FOLLOWEDBY
// wins over any shorter match at the same position.
var boolKeywords = []string{"FOLLOWEDBY", "AND", "OR"}

// splitComparisons splits s on AND/OR/FOLLOWEDBY (case-insensitive) that
// appear outside quotes, at any bracket depth. Quoted values containing
// those words ("https://or.example.com/") are left intact, as are words
// merely containing a keyword ("network", "ERROR").
func splitComparisons(s string) []string {
	var parts []string
	var quote byte // 0 when outside quotes, otherwise '\'' or '"'
	start := 0
	i := 0
	for i < len(s) {
		c := s[i]
		if quote != 0 {
			if c == '\\' {
				i += 2 // value content is opaque; skip escaped char
				continue
			}
			if c == quote {
				quote = 0
			}
			i++
			continue
		}
		if c == '\'' || c == '"' {
			quote = c
			i++
			continue
		}
		if n := matchBoolKeyword(s, i); n > 0 {
			parts = append(parts, s[start:i])
			i += n
			start = i
			continue
		}
		i++
	}
	return append(parts, s[start:])
}

// matchBoolKeyword reports the length of the boolean keyword starting at
// s[i], or 0. A keyword needs word boundaries on both sides (anything but
// [A-Za-z0-9_], or the string ends).
func matchBoolKeyword(s string, i int) int {
	for _, kw := range boolKeywords {
		if i+len(kw) > len(s) {
			continue
		}
		match := true
		for j := 0; j < len(kw); j++ {
			if toUpper(s[i+j]) != kw[j] {
				match = false
				break
			}
		}
		if !match {
			continue
		}
		if i > 0 && isKwChar(s[i-1]) {
			continue
		}
		if i+len(kw) < len(s) && isKwChar(s[i+len(kw)]) {
			continue
		}
		return len(kw)
	}
	return 0
}

func isKwChar(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_'
}

func toUpper(c byte) byte {
	if c >= 'a' && c <= 'z' {
		return c - ('a' - 'A')
	}
	return c
}

// parseComparison attempts one combinator-split piece as a supported
// comparison. Leading parens/brackets are stripped and anything after the
// value's closing quote ("]", qualifiers, junk) is ignored.
func parseComparison(piece string) (Observable, bool) {
	s := strings.TrimSpace(piece)
	for len(s) > 0 && (s[0] == '(' || s[0] == '[') {
		s = strings.TrimSpace(s[1:])
	}
	// First "=" outside quotes is the comparator; the LHS never contains
	// one, while a quoted value may (e.g. a URL query string).
	eq := indexUnquoted(s, '=')
	if eq < 0 {
		return Observable{}, false
	}
	lhs := strings.TrimSpace(s[:eq])
	if lhs == "" {
		return Observable{}, false
	}
	// Reject !=, <=, >=: a bare "=" has no operator char before it.
	if last := lhs[len(lhs)-1]; last == '!' || last == '<' || last == '>' {
		return Observable{}, false
	}
	// The value must be quoted: rejects ==, =>, and unquoted values
	// like [file:size = 100].
	rest := strings.TrimSpace(s[eq+1:])
	if len(rest) < 2 || rest[0] != '\'' && rest[0] != '"' {
		return Observable{}, false
	}
	val, goodVal := quotedPrefix(rest)
	if !goodVal {
		return Observable{}, false
	}
	typ, goodPath := observableType(lhs)
	if !goodPath {
		return Observable{}, false
	}
	return Observable{Type: typ, Value: val}, true
}

// indexUnquoted returns the index of the first target byte outside quotes,
// or -1.
func indexUnquoted(s string, target byte) int {
	var quote byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			if c == '\\' {
				i++
				continue
			}
			if c == quote {
				quote = 0
			}
			continue
		}
		if c == '\'' || c == '"' {
			quote = c
			continue
		}
		if c == target {
			return i
		}
	}
	return -1
}

// quotedPrefix returns the literal contents of the quoted string at the
// start of s. Content after the closing quote is the caller's business.
func quotedPrefix(s string) (string, bool) {
	q := s[0]
	for i := 1; i < len(s); i++ {
		if s[i] == '\\' {
			i++ // value content is opaque; skip escaped char
			continue
		}
		if s[i] == q {
			return s[1:i], true
		}
	}
	return "", false
}

// observableType maps a supported object path to its Observable type token.
func observableType(lhs string) (string, bool) {
	// Allow whitespace around the ":" separators.
	noSpace := strings.Join(strings.Fields(lhs), "")
	obj, prop, found := strings.Cut(noSpace, ":")
	if !found {
		return "", false
	}
	switch obj {
	case "file":
		field, arg, hasArg := strings.Cut(prop, ".")
		switch {
		case field == "name" && !hasArg:
			return "file-name", true
		case field == "hashes" && hasArg:
			switch strings.ToUpper(strings.Trim(arg, `'"`)) {
			case "SHA-256":
				return "file-hash-sha256", true
			case "SHA-1":
				return "file-hash-sha1", true
			case "MD5":
				return "file-hash-md5", true
			}
			return "", false
		default:
			return "", false
		}
	case "domain-name", "ipv4-addr", "ipv6-addr", "url":
		if prop != "value" {
			return "", false
		}
		switch obj {
		case "domain-name":
			return "domain", true
		case "ipv4-addr":
			return "ipv4", true
		case "ipv6-addr":
			return "ipv6", true
		default:
			return "url", true
		}
	default:
		return "", false
	}
}
