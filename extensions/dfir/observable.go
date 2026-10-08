package dfir

import "strings"

// Observable is an atomic, canonicalizable indicator value extracted from a
// STIX pattern or supplied by a DFIR case.
type Observable struct {
	Type  string `json:"type"`  // stable type tokens: "domain", "ipv4", "ipv6", "url", "file-name", "file-hash-sha256", "file-hash-sha1", "file-hash-md5"
	Value string `json:"value"` // raw value as found; use Canonical() for matching
}

// Canonicalization rules (used for cross-source matching):
//   - domain → lowercase; strip one trailing "."
//   - file-hash-sha256 / -sha1 / -md5 → lowercase; strip all whitespace
//   - ipv4 / ipv6 → trim leading/trailing whitespace (no other normalization)
//   - url → trim leading/trailing whitespace only (paths are case-sensitive)
//   - file-name → trim leading/trailing whitespace only (filesystems can be case-sensitive)
//   - unknown types → trim leading/trailing whitespace (safe default)
//
// Canonical returns the normalized form used for cross-source matching.
func (o Observable) Canonical() string {
	switch o.Type {
	case "domain":
		v := strings.ToLower(strings.TrimSpace(o.Value))
		// Strip a single trailing dot (root marker): "example.com." -> "example.com",
		// but ".." -> "." (only one dot removed).
		return strings.TrimSuffix(v, ".")
	case "file-hash-sha256", "file-hash-sha1", "file-hash-md5":
		// Hashes never legitimately contain whitespace; drop it all so
		// pasted chunked values ("ab cd") still match.
		return strings.Join(strings.Fields(strings.ToLower(o.Value)), "")
	case "ipv4", "ipv6", "url", "file-name":
		return strings.TrimSpace(o.Value)
	default:
		return strings.TrimSpace(o.Value)
	}
}

// Key returns a match key combining type and canonical value.
// The NUL separator keeps values containing "|" from colliding.
func (o Observable) Key() string {
	return o.Type + "\x00" + o.Canonical()
}
