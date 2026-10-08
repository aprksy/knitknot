package dfir

import (
	"fmt"

	"github.com/aprksy/knitknot/pkg/ports/storage"
)

// WorkspaceOptions tunes BuildWorkspace.
type WorkspaceOptions struct {
	// CaseSource is the source_feed recorded on case observable nodes
	// and the based-on edges inducing them.
	CaseSource string
	// AllObservables materializes the observable layer for every
	// indicator in src, not just the case-matched ones.
	AllObservables bool
}

// WorkspaceStats counts what BuildWorkspace materialized into dst.
type WorkspaceStats struct {
	Observables int
	Indicators  int
	Malware     int
	Campaigns   int
	Actors      int
	TTPs        int
	Edges       int
}

// contextKinds are the CTI edge kinds copied into the workspace: the links
// between matched indicators and their reachable adversary context.
var contextKinds = map[string]bool{
	EdgeIndicates:    true,
	EdgeUses:         true,
	EdgeAttributedTo: true,
}

// BuildWorkspace writes a projected case workspace into dst: case observables,
// the indicators that match them, the CTI context reachable from those
// indicators, and the inducing edges. src is read-only (never mutated).
//
// It reuses Correlate to find matches. Observable nodes are deduped by
// type+canonical value across the whole build; copied CTI nodes keep their
// Label and Props (maps cloned, never aliased); dst assigns fresh IDs with
// endpoints remapped.
// BuildWorkspace writes a projected case workspace into dst: case observables,
// the indicators that match them, the CTI context reachable from those
// indicators, and the inducing edges. src is read-only (never mutated).
//
// With WorkspaceOptions.AllObservables, the observable layer is materialized
// for every indicator in src: each observable-bearing indicator is copied
// along with its pattern observables and based-on edges. Indicators whose
// patterns extract no supported observable are skipped, keeping the layer
// thin and every based-on edge non-dangling.
//
// It reuses Correlate to find matches. Observable nodes are deduped by
// type+canonical value across the whole build; copied CTI nodes keep their
// Label and Props (maps cloned, never aliased); dst assigns fresh IDs with
// endpoints remapped.
func BuildWorkspace(src, dst storage.StorageEngine, caseObs []CaseObservable, opts WorkspaceOptions) (WorkspaceStats, error) {
	var stats WorkspaceStats
	matches := Correlate(src, caseObs)

	idMap := make(map[string]string) // src node ID -> dst node ID
	copyID := func(srcID string, counter *int) (string, error) {
		if id, ok := idMap[srcID]; ok {
			return id, nil
		}
		n, ok := src.GetNode(srcID)
		if !ok {
			return "", fmt.Errorf("dfir: workspace: source node %q not found", srcID)
		}
		id, err := dst.AddNode(n.Label, cloneProps(n.Props))
		if err != nil {
			return "", fmt.Errorf("dfir: workspace: add node: %w", err)
		}
		idMap[srcID] = id
		*counter++
		return id, nil
	}
	copyRef := func(ref NodeRef, counter *int) (string, error) {
		return copyID(ref.ID, counter)
	}

	obsNodes := make(map[string]string) // Observable.Key() -> dst node ID
	ensureObservable := func(obs Observable) (string, error) {
		key := obs.Key()
		if id, ok := obsNodes[key]; ok {
			return id, nil
		}
		canon := obs.Canonical()
		id, err := dst.AddNode(LabelObservable, map[string]any{
			"type":         obs.Type,
			"value":        canon,
			"name":         canon, // MatchOn=name so Has('based-on', value) resolves
			PropSourceFeed: opts.CaseSource,
		})
		if err != nil {
			return "", fmt.Errorf("dfir: workspace: add observable: %w", err)
		}
		obsNodes[key] = id
		stats.Observables++
		return id, nil
	}
	type induction struct{ fromSrc, obsKey string }
	var inductions []induction
	induced := make(map[induction]bool)
	induce := func(indSrcID, obsKey string) {
		in := induction{fromSrc: indSrcID, obsKey: obsKey}
		if !induced[in] {
			induced[in] = true
			inductions = append(inductions, in)
		}
	}
	for _, m := range matches {
		key := m.Observable.Key()
		if _, err := ensureObservable(m.Observable); err != nil {
			return stats, err
		}
		for _, ind := range m.Indicators {
			if _, err := copyRef(ind, &stats.Indicators); err != nil {
				return stats, err
			}
			induce(ind.ID, key)
		}
		for _, mal := range m.Malware {
			if _, err := copyRef(mal, &stats.Malware); err != nil {
				return stats, err
			}
		}
		for _, camp := range m.Campaigns {
			if _, err := copyRef(camp, &stats.Campaigns); err != nil {
				return stats, err
			}
		}
		for _, actor := range m.Actors {
			if _, err := copyRef(actor, &stats.Actors); err != nil {
				return stats, err
			}
		}
		for _, ttp := range m.TTPs {
			if _, err := copyRef(ttp, &stats.TTPs); err != nil {
				return stats, err
			}
		}
	}

	if opts.AllObservables {
		for _, ind := range src.GetNodesByLabel(LabelIndicator) {
			pat, _ := ind.Props["pattern"].(string)
			if pat == "" {
				continue
			}
			obs, ok := ExtractObservables(pat)
			if !ok {
				continue
			}
			if _, err := copyID(ind.ID, &stats.Indicators); err != nil {
				return stats, err
			}
			for _, o := range obs {
				if _, err := ensureObservable(o); err != nil {
					return stats, err
				}
				induce(ind.ID, o.Key())
			}
		}
	}

	// Inducing edges: matched indicator -based-on-> observable.
	for _, in := range inductions {
		if err := dst.AddEdge(idMap[in.fromSrc], obsNodes[in.obsKey], EdgeBasedOn, map[string]any{PropSourceFeed: opts.CaseSource}); err != nil {
			return stats, fmt.Errorf("dfir: workspace: add based-on edge: %w", err)
		}
		stats.Edges++
	}

	// Context edges: single pass over src keeps dedup inherent; only edges
	// with both endpoints in the projection are copied.
	for _, e := range src.GetAllEdges() {
		if !contextKinds[e.Kind] {
			continue
		}
		from, ok1 := idMap[e.From]
		to, ok2 := idMap[e.To]
		if !ok1 || !ok2 {
			continue
		}
		if err := dst.AddEdge(from, to, e.Kind, cloneProps(e.Props)); err != nil {
			return stats, fmt.Errorf("dfir: workspace: add edge: %w", err)
		}
		stats.Edges++
	}
	return stats, nil
}

func cloneProps(p map[string]any) map[string]any {
	out := make(map[string]any, len(p))
	for k, v := range p {
		out[k] = v
	}
	return out
}
