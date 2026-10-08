package dfir

import (
	"fmt"

	"github.com/aprksy/knitknot/extensions/cti"
	"github.com/aprksy/knitknot/pkg/ports/storage"
)

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
	cti.EdgeIndicates:    true,
	cti.EdgeUses:         true,
	cti.EdgeAttributedTo: true,
}

// BuildWorkspace writes a projected case workspace into dst: case observables,
// the indicators that match them, the CTI context reachable from those
// indicators, and the inducing edges. src is read-only (never mutated).
//
// It reuses Correlate to find matches. Observable nodes are deduped by
// type+canonical value across the whole build; copied CTI nodes keep their
// Label and Props (maps cloned, never aliased); dst assigns fresh IDs with
// endpoints remapped.
func BuildWorkspace(src, dst storage.StorageEngine, caseObs []CaseObservable, caseSource string) (WorkspaceStats, error) {
	var stats WorkspaceStats
	matches := Correlate(src, caseObs)

	idMap := make(map[string]string) // src node ID -> dst node ID
	copyRef := func(ref NodeRef, counter *int) (string, error) {
		if id, ok := idMap[ref.ID]; ok {
			return id, nil
		}
		n, ok := src.GetNode(ref.ID)
		if !ok {
			return "", fmt.Errorf("dfir: workspace: source node %q not found", ref.ID)
		}
		id, err := dst.AddNode(n.Label, cloneProps(n.Props))
		if err != nil {
			return "", fmt.Errorf("dfir: workspace: add node: %w", err)
		}
		idMap[ref.ID] = id
		*counter++
		return id, nil
	}

	obsNodes := make(map[string]string) // Observable.Key() -> dst node ID
	type induction struct{ fromSrc, obsKey string }
	var inductions []induction
	induced := make(map[induction]bool)
	for _, m := range matches {
		key := m.Observable.Key()
		if _, ok := obsNodes[key]; !ok {
			canon := m.Observable.Canonical()
			id, err := dst.AddNode(cti.LabelObservable, map[string]any{
				"type":         m.Observable.Type,
				"value":        canon,
				"name":         canon, // MatchOn=name so Has('based-on', value) resolves
				cti.SourceFeed: caseSource,
			})
			if err != nil {
				return stats, fmt.Errorf("dfir: workspace: add observable: %w", err)
			}
			obsNodes[key] = id
			stats.Observables++
		}
		for _, ind := range m.Indicators {
			if _, err := copyRef(ind, &stats.Indicators); err != nil {
				return stats, err
			}
			in := induction{fromSrc: ind.ID, obsKey: key}
			if !induced[in] {
				induced[in] = true
				inductions = append(inductions, in)
			}
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

	// Inducing edges: matched indicator -based-on-> observable.
	for _, in := range inductions {
		if err := dst.AddEdge(idMap[in.fromSrc], obsNodes[in.obsKey], cti.EdgeBasedOn, map[string]any{cti.SourceFeed: caseSource}); err != nil {
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
