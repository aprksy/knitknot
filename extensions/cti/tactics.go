// ext: cti-tactics — opt-in tactic materialization (ADR 0005, delivery stage 1).
//
// Turns ATT&CK kill_chain_phases (otherwise an inert array prop) into
// first-class graph structure: x-mitre-tactic nodes carrying a rank, linked
// from attack-patterns by has-tactic edges. Runs only when
// ImportContext.MaterializeTactics is set; the default import is untouched.
package cti

import (
	"context"
	"time"

	"github.com/aprksy/knitknot/extensions"
	"github.com/aprksy/knitknot/pkg/ports/storage"
)

// fallbackTactic is one row of the fixed canonical tactic table. // ext: cti-tactics
type fallbackTactic struct { // ext: cti-tactics
	shortname string // ext: cti-tactics
	name      string // ext: cti-tactics
} // ext: cti-tactics

// fallbackTactics mirrors x-mitre-matrix.tactic_refs in the verified canonical
// order and is only used when no matrix is present. Version it against ATT&CK
// releases; prefer the matrix whenever one exists. // ext: cti-tactics
var fallbackTactics = []fallbackTactic{ // ext: cti-tactics
	{"reconnaissance", "Reconnaissance"},
	{"resource-development", "Resource Development"},
	{"initial-access", "Initial Access"},
	{"execution", "Execution"},
	{"persistence", "Persistence"},
	{"privilege-escalation", "Privilege Escalation"},
	{"stealth", "Stealth"},
	{"defense-impairment", "Defense Impairment"},
	{"credential-access", "Credential Access"},
	{"discovery", "Discovery"},
	{"lateral-movement", "Lateral Movement"},
	{"collection", "Collection"},
	{"command-and-control", "Command and Control"},
	{"exfiltration", "Exfiltration"},
	{"impact", "Impact"},
} // ext: cti-tactics

// mitreAttackChain is the only kill_chain_name that gets ranked. // ext: cti-tactics
const mitreAttackChain = "mitre-attack" // ext: cti-tactics

// materializeTactics builds tactic nodes and has-tactic edges after a normal
// import. It returns the number of tactic nodes created or updated and the
// number of has-tactic edges created. Stable identity is x_mitre_shortname,
// so re-running merges instead of duplicating. // ext: cti-tactics
func materializeTactics(ctx context.Context, s storage.StorageEngine, ic extension.ImportContext) (tactics, edges int, err error) { // ext: cti-tactics
	rankOf := tacticRanks(s) // ext: cti-tactics

	now := time.Now().UTC().Format("2006-01-02T15:04:05Z07:00") // ext: cti-tactics

	// Index imported tactic nodes by shortname and stamp their rank. // ext: cti-tactics
	tacticID := make(map[string]string, 16)            // ext: cti-tactics — shortname -> graph ID
	for _, n := range s.GetNodesByLabel(LabelTactic) { // ext: cti-tactics
		short, _ := n.Props[PropShortname].(string) // ext: cti-tactics
		if short == "" {                            // ext: cti-tactics
			continue // ext: cti-tactics — not a recognizable tactic, leave alone
		} // ext: cti-tactics
		tacticID[short] = n.ID          // ext: cti-tactics
		want := map[string]any{}        // ext: cti-tactics
		if r, ok := rankOf[short]; ok { // ext: cti-tactics
			want[PropRank] = r // ext: cti-tactics
		} // ext: cti-tactics
		if name, _ := n.Props["name"].(string); name == "" { // ext: cti-tactics
			if disp := fallbackName(short); disp != "" { // ext: cti-tactics
				want["name"] = disp // ext: cti-tactics
			} // ext: cti-tactics
		} // ext: cti-tactics
		if cur, _ := n.Props[PropExternalID].(string); cur == "" { // ext: cti-tactics
			if ext := externalIDOf(n.Props); ext != "" { // ext: cti-tactics
				want[PropExternalID] = ext // ext: cti-tactics
			} // ext: cti-tactics
		} // ext: cti-tactics
		if len(want) == 0 || propsCover(n.Props, want) { // ext: cti-tactics
			continue // ext: cti-tactics — nothing new; keeps re-runs snapshot-quiet
		} // ext: cti-tactics
		if err := updateNodeSrc(s, n.ID, withProvenance(mergeProps(n.Props, want), ic, now), ic.Source, ic.Transaction); err != nil { // ext: cti-tactics
			return 0, 0, err // ext: cti-tactics
		} // ext: cti-tactics
		tactics++ // ext: cti-tactics
	} // ext: cti-tactics

	// Link every attack-pattern to its tactics, one edge per phase. // ext: cti-tactics
	for i, p := range s.GetNodesByLabel(LabelAttackPattern) { // ext: cti-tactics
		if i%64 == 0 { // ext: cti-tactics
			if err := ctx.Err(); err != nil { // ext: cti-tactics
				return 0, 0, err // ext: cti-tactics
			} // ext: cti-tactics
		} // ext: cti-tactics
		phases := tacticShortnames(p.Props[PropKillChainPhases]) // ext: cti-tactics
		if len(phases) == 0 {                                    // ext: cti-tactics
			continue // ext: cti-tactics — no ATT&CK phases, nothing to link
		} // ext: cti-tactics
		linked := make(map[string]bool, 4)       // ext: cti-tactics — tactic graph IDs already edged
		for _, e := range s.GetEdgesFrom(p.ID) { // ext: cti-tactics
			if e.Kind == EdgeHasTactic { // ext: cti-tactics
				linked[e.To] = true // ext: cti-tactics
			} // ext: cti-tactics
		} // ext: cti-tactics
		for _, short := range phases { // ext: cti-tactics
			tid, ok := tacticID[short] // ext: cti-tactics
			if !ok {                   // ext: cti-tactics — no tactic object: synthesize a minimal node
				props := map[string]any{ // ext: cti-tactics
					PropType:        LabelTactic,           // ext: cti-tactics
					PropShortname:   short,                 // ext: cti-tactics
					"name":          fallbackNameOr(short), // ext: cti-tactics
					PropSynthesized: true,                  // ext: cti-tactics
				} // ext: cti-tactics
				if r, ok := rankOf[short]; ok { // ext: cti-tactics
					props[PropRank] = r // ext: cti-tactics
				} // ext: cti-tactics
				// No stix_id: never fabricate STIX IDs; export skips such nodes.
				gid, err := addNodeSrc(s, LabelTactic, withProvenance(props, ic, now), ic.Source, ic.Transaction) // ext: cti-tactics
				if err != nil {                                                                                   // ext: cti-tactics
					return 0, 0, err // ext: cti-tactics
				} // ext: cti-tactics
				tid = gid             // ext: cti-tactics
				tacticID[short] = gid // ext: cti-tactics
				tactics++             // ext: cti-tactics
			} // ext: cti-tactics
			if linked[tid] { // ext: cti-tactics
				continue // ext: cti-tactics — idempotent: edge already exists
			} // ext: cti-tactics
			// No stix_id: export skips non-STIX edges (counted, never fabricated).
			if err := addEdgeSrc(s, p.ID, tid, EdgeHasTactic, withProvenance(map[string]any{}, ic, now), ic.Source, ic.Transaction); err != nil { // ext: cti-tactics
				return 0, 0, err // ext: cti-tactics
			} // ext: cti-tactics
			linked[tid] = true // ext: cti-tactics
			edges++            // ext: cti-tactics
		} // ext: cti-tactics
	} // ext: cti-tactics
	return tactics, edges, nil // ext: cti-tactics
} // ext: cti-tactics

// tacticRanks maps shortname -> 1-based rank from every x-mitre-matrix's
// tactic_refs (resolving STIX IDs through imported tactic objects). With no
// matrix or no resolvable refs it falls back to the fixed table; gaps in a
// partial matrix are filled from the fallback too. // ext: cti-tactics
func tacticRanks(s storage.StorageEngine) map[string]int { // ext: cti-tactics
	idToShort := make(map[string]string, 16)           // ext: cti-tactics
	for _, n := range s.GetNodesByLabel(LabelTactic) { // ext: cti-tactics
		sid, _ := n.Props[StixID].(string)          // ext: cti-tactics
		short, _ := n.Props[PropShortname].(string) // ext: cti-tactics
		if sid != "" && short != "" {               // ext: cti-tactics
			idToShort[sid] = short // ext: cti-tactics
		} // ext: cti-tactics
	} // ext: cti-tactics
	rankOf := make(map[string]int, 16)                 // ext: cti-tactics
	for _, m := range s.GetNodesByLabel(LabelMatrix) { // ext: cti-tactics
		for i, ref := range asStringSlice(m.Props[PropTacticRefs]) { // ext: cti-tactics
			short := idToShort[ref] // ext: cti-tactics — "" when the tactic object is absent
			if short == "" {        // ext: cti-tactics
				continue // ext: cti-tactics
			} // ext: cti-tactics
			if _, ok := rankOf[short]; !ok { // ext: cti-tactics — first matrix wins
				rankOf[short] = i + 1 // ext: cti-tactics — 1-based
			} // ext: cti-tactics
		} // ext: cti-tactics
	} // ext: cti-tactics
	for i, t := range fallbackTactics { // ext: cti-tactics
		if _, ok := rankOf[t.shortname]; !ok { // ext: cti-tactics
			rankOf[t.shortname] = i + 1 // ext: cti-tactics — fallback fills gaps or the whole map
		} // ext: cti-tactics
	} // ext: cti-tactics
	return rankOf // ext: cti-tactics
} // ext: cti-tactics

// tacticShortnames extracts deduped mitre-attack phase_names from a flattened
// kill_chain_phases value. Absent or malformed shapes yield nothing, never a
// panic. // ext: cti-tactics
func tacticShortnames(v any) []string { // ext: cti-tactics
	arr, ok := v.([]any) // ext: cti-tactics
	if !ok {             // ext: cti-tactics
		return nil // ext: cti-tactics
	} // ext: cti-tactics
	var out []string                 // ext: cti-tactics
	seen := make(map[string]bool, 4) // ext: cti-tactics
	for _, item := range arr {       // ext: cti-tactics
		m, ok := item.(map[string]any) // ext: cti-tactics
		if !ok {                       // ext: cti-tactics
			continue // ext: cti-tactics
		} // ext: cti-tactics
		if name, _ := m["kill_chain_name"].(string); name != mitreAttackChain { // ext: cti-tactics
			continue // ext: cti-tactics — only mitre-attack is ranked
		} // ext: cti-tactics
		phase, _ := m["phase_name"].(string) // ext: cti-tactics
		if phase == "" || seen[phase] {      // ext: cti-tactics
			continue // ext: cti-tactics
		} // ext: cti-tactics
		seen[phase] = true       // ext: cti-tactics
		out = append(out, phase) // ext: cti-tactics
	} // ext: cti-tactics
	return out // ext: cti-tactics
} // ext: cti-tactics

// asStringSlice safely flattens a JSON array prop to strings. // ext: cti-tactics
func asStringSlice(v any) []string { // ext: cti-tactics
	arr, ok := v.([]any) // ext: cti-tactics
	if !ok {             // ext: cti-tactics
		return nil // ext: cti-tactics
	} // ext: cti-tactics
	var out []string           // ext: cti-tactics
	for _, item := range arr { // ext: cti-tactics
		if s, ok := item.(string); ok && s != "" { // ext: cti-tactics
			out = append(out, s) // ext: cti-tactics
		} // ext: cti-tactics
	} // ext: cti-tactics
	return out // ext: cti-tactics
} // ext: cti-tactics

// externalIDOf returns the first external_id in flattened external_references. // ext: cti-tactics
func externalIDOf(props map[string]any) string { // ext: cti-tactics
	arr, ok := props[PropExternalRefs].([]any) // ext: cti-tactics
	if !ok {                                   // ext: cti-tactics
		return "" // ext: cti-tactics
	} // ext: cti-tactics
	for _, item := range arr { // ext: cti-tactics
		m, ok := item.(map[string]any) // ext: cti-tactics
		if !ok {                       // ext: cti-tactics
			continue // ext: cti-tactics
		} // ext: cti-tactics
		if id, _ := m["external_id"].(string); id != "" { // ext: cti-tactics
			return id // ext: cti-tactics
		} // ext: cti-tactics
	} // ext: cti-tactics
	return "" // ext: cti-tactics
} // ext: cti-tactics

// fallbackName returns the display name for a known shortname, else "". // ext: cti-tactics
func fallbackName(short string) string { // ext: cti-tactics
	for _, t := range fallbackTactics { // ext: cti-tactics
		if t.shortname == short { // ext: cti-tactics
			return t.name // ext: cti-tactics
		} // ext: cti-tactics
	} // ext: cti-tactics
	return "" // ext: cti-tactics
} // ext: cti-tactics

// fallbackNameOr returns the display name, defaulting to the shortname itself
// for tactics the fallback table does not know (yet). // ext: cti-tactics
func fallbackNameOr(short string) string { // ext: cti-tactics
	if n := fallbackName(short); n != "" { // ext: cti-tactics
		return n // ext: cti-tactics
	} // ext: cti-tactics
	return short // ext: cti-tactics
} // ext: cti-tactics

// withProvenance stamps source_feed/imported_at like the main import path. // ext: cti-tactics
func withProvenance(props map[string]any, ic extension.ImportContext, now string) map[string]any { // ext: cti-tactics
	props[SourceFeed] = ic.Source // ext: cti-tactics
	props[ImportedAt] = now       // ext: cti-tactics
	return props                  // ext: cti-tactics
} // ext: cti-tactics

// propsCover reports whether have already carries every key/value in want
// (numeric-tolerant, so JSON/gob round-trips don't churn snapshots). // ext: cti-tactics
func propsCover(have, want map[string]any) bool { // ext: cti-tactics
	for k, w := range want { // ext: cti-tactics
		h, ok := have[k] // ext: cti-tactics
		if !ok {         // ext: cti-tactics
			return false // ext: cti-tactics
		} // ext: cti-tactics
		if wn, ok := toFloat(w); ok { // ext: cti-tactics
			hn, ok := toFloat(h) // ext: cti-tactics
			if !ok || hn != wn { // ext: cti-tactics
				return false // ext: cti-tactics
			} // ext: cti-tactics
			continue // ext: cti-tactics
		} // ext: cti-tactics
		if h != w { // ext: cti-tactics
			return false // ext: cti-tactics
		} // ext: cti-tactics
	} // ext: cti-tactics
	return true // ext: cti-tactics
} // ext: cti-tactics

// toFloat normalizes ints/floats across JSON (float64) and gob (int). // ext: cti-tactics
func toFloat(v any) (float64, bool) { // ext: cti-tactics
	switch n := v.(type) { // ext: cti-tactics
	case int: // ext: cti-tactics
		return float64(n), true // ext: cti-tactics
	case int64: // ext: cti-tactics
		return float64(n), true // ext: cti-tactics
	case float64: // ext: cti-tactics
		return n, true // ext: cti-tactics
	case float32: // ext: cti-tactics
		return float64(n), true // ext: cti-tactics
	} // ext: cti-tactics
	return 0, false // ext: cti-tactics
} // ext: cti-tactics
