package dfir

import (
	"encoding/csv"
	"fmt"
	"io"
	"strings"
)

// CaseObservable is a DFIR case input row: an observable plus optional
// free-text context carried through to the report.
type CaseObservable struct {
	Observable
	Context string `json:"context,omitempty"` // optional, carried through to the report
}

// caseTypeAliases maps lowercased input type tokens to canonical
// Observable.Type tokens.
var caseTypeAliases = map[string]string{
	"sha256": "file-hash-sha256", "sha-256": "file-hash-sha256", "file-hash-sha256": "file-hash-sha256",
	"sha1": "file-hash-sha1", "sha-1": "file-hash-sha1", "file-hash-sha1": "file-hash-sha1",
	"md5": "file-hash-md5", "file-hash-md5": "file-hash-md5",
	"domain": "domain", "domain-name": "domain",
	"ip": "ipv4", "ipv4": "ipv4", "ipv4-addr": "ipv4",
	"ipv6": "ipv6", "ipv6-addr": "ipv6",
	"url":      "url",
	"filename": "file-name", "file-name": "file-name", "name": "file-name",
}

// ParseCaseCSV reads a header row followed by `type,value[,context]` rows.
// Header names are case-insensitive; `context` is optional free text.
// Blank lines are skipped. Unknown types and rows missing type or value
// are errors naming the row — case data is never silently dropped.
func ParseCaseCSV(r io.Reader) ([]CaseObservable, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1 // tolerate ragged rows; we validate by header index
	cr.TrimLeadingSpace = true

	header, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("case csv: read header: %w", err)
	}
	typeIdx, valueIdx, ctxIdx := -1, -1, -1
	for i, h := range header {
		switch strings.ToLower(strings.TrimSpace(h)) {
		case "type":
			typeIdx = i
		case "value":
			valueIdx = i
		case "context":
			ctxIdx = i
		}
	}
	if typeIdx < 0 || valueIdx < 0 {
		return nil, fmt.Errorf("case csv: header must contain type and value columns, got %q", strings.Join(header, ","))
	}

	var out []CaseObservable
	row := 1 // header is row 1
	for {
		rec, err := cr.Read()
		if err == io.EOF {
			return out, nil
		}
		row++
		if err != nil {
			return nil, fmt.Errorf("case csv row %d: %w", row, err)
		}
		if isBlankRecord(rec) {
			continue
		}
		typ, val := fieldAt(rec, typeIdx), fieldAt(rec, valueIdx)
		if strings.TrimSpace(typ) == "" || strings.TrimSpace(val) == "" {
			return nil, fmt.Errorf("case csv row %d: missing type or value", row)
		}
		canon, ok := caseTypeAliases[strings.ToLower(strings.TrimSpace(typ))]
		if !ok {
			return nil, fmt.Errorf("case csv row %d: unknown type %q", row, strings.TrimSpace(typ))
		}
		var ctx string
		if ctxIdx >= 0 {
			ctx = fieldAt(rec, ctxIdx)
		}
		out = append(out, CaseObservable{
			Observable: Observable{Type: canon, Value: strings.TrimSpace(val)},
			Context:    strings.TrimSpace(ctx),
		})
	}
}

func fieldAt(rec []string, i int) string {
	if i < len(rec) {
		return rec[i]
	}
	return ""
}

func isBlankRecord(rec []string) bool {
	for _, f := range rec {
		if strings.TrimSpace(f) != "" {
			return false
		}
	}
	return true
}
