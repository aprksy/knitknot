// ext: cti-identity — STIX ID parsing and observable canonicalization.
package cti

import (
	"errors"
	"regexp"
	"strings"
)

var (
	// ext: cti-identity — STIX types are lowercase letters, digits, hyphens.
	stixTypeRe = regexp.MustCompile(`^[a-z0-9-]+$`)
	// ext: cti-identity — canonical 8-4-4-4-12 lowercase-hex UUID.
	uuidRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	hexRe  = regexp.MustCompile(`^[0-9a-f]+$`)
	cveRe  = regexp.MustCompile(`^CVE-\d{4}-\d{4,}$`)
)

// ParseSTIXID splits a STIX ID of the form <type>--<UUID>.
// ext: cti-identity — STIX object types never contain "--", so split on the first occurrence.
func ParseSTIXID(id string) (objType, uuid string, err error) {
	idx := strings.Index(id, "--")
	if idx < 0 {
		return "", "", errors.New("cti: invalid STIX ID: missing \"--\" separator")
	}
	objType, uuid = id[:idx], id[idx+2:]
	if objType == "" {
		return "", "", errors.New("cti: invalid STIX ID: empty object type")
	}
	if !stixTypeRe.MatchString(objType) {
		return "", "", errors.New("cti: invalid STIX ID: object type must be lowercase letters, digits, or hyphens")
	}
	if uuid == "" {
		return "", "", errors.New("cti: invalid STIX ID: empty UUID")
	}
	if !uuidRe.MatchString(uuid) {
		return "", "", errors.New("cti: invalid STIX ID: malformed UUID")
	}
	return objType, uuid, nil
}

// CanonicalizeDomain lowercases s and trims ONE trailing dot.
// ext: cti-identity — empty input maps to empty output.
func CanonicalizeDomain(s string) string {
	s = strings.ToLower(s)
	if len(s) > 1 && strings.HasSuffix(s, ".") {
		s = s[:len(s)-1]
	}
	return s
}

// CanonicalizeHash normalizes a file-hash digest to lowercase hex.
// ext: cti-identity — key composition (algorithm:digest) lives with the importer lane.
func CanonicalizeHash(algorithm, digest string) (string, error) {
	if strings.TrimSpace(algorithm) == "" {
		return "", errors.New("cti: hash algorithm must not be empty")
	}
	digest = strings.ToLower(strings.TrimSpace(digest))
	if digest == "" {
		return "", errors.New("cti: hash digest must not be empty")
	}
	if !hexRe.MatchString(digest) {
		return "", errors.New("cti: hash digest must be hex")
	}
	return digest, nil
}

// CanonicalizeCVE normalizes s to the uppercase canonical CVE identifier.
func CanonicalizeCVE(s string) (string, error) {
	s = strings.ToUpper(strings.TrimSpace(s))
	if !cveRe.MatchString(s) {
		return "", errors.New("cti: invalid CVE identifier")
	}
	return s, nil
}

// CanonicalizeURL lowercases only the scheme+host part, preserving path/query case.
// ext: cti-identity — minimal normalization; never guess away meaningful parts.
func CanonicalizeURL(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", errors.New("cti: URL must not be empty")
	}
	parts := strings.SplitN(s, "://", 2)
	if len(parts) != 2 {
		return s, nil
	}
	rest := parts[1]
	host, suffix := rest, ""
	if i := strings.Index(rest, "/"); i >= 0 {
		host, suffix = rest[:i], rest[i:]
	}
	return strings.ToLower(parts[0]) + "://" + strings.ToLower(host) + suffix, nil
}
