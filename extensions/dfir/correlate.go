package dfir

import (
	"github.com/aprksy/knitknot/extensions/cti"
	"github.com/aprksy/knitknot/pkg/ports/storage"
	"github.com/aprksy/knitknot/pkg/ports/types"
)

// NodeRef is a graph node referenced from a Match: its graph identity plus
// the display fields a report needs.
type NodeRef struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Name   string `json:"name"`
	StixID string `json:"stix_id"`
}

// Match is one case observable found in the CTI graph plus the adversary
// context reachable from the matching indicator(s).
type Match struct {
	Observable Observable `json:"observable"`
	Context    string     `json:"context,omitempty"`
	Indicators []NodeRef  `json:"indicators"`
	Malware    []NodeRef  `json:"malware"`
	Campaigns  []NodeRef  `json:"campaigns"`
	Actors     []NodeRef  `json:"actors"`
	TTPs       []NodeRef  `json:"ttps"`
}

// Correlate matches case observables against indicator patterns in the
// graph and walks to the reachable adversary context.
//
// The indicator index maps Observable.Key() to every indicator whose
// pattern carries that observable; indicators with unsupported patterns
// are skipped. One Match is returned per case observable that hit, in
// input order; a miss yields no entry. Every ref list is deduped by node.
func Correlate(s storage.StorageEngine, caseObs []CaseObservable) []Match {
	byKey := make(map[string][]*types.Node)
	for _, n := range s.GetNodesByLabel(cti.LabelIndicator) {
		pat, _ := n.Props["pattern"].(string)
		if pat == "" {
			continue
		}
		obs, ok := ExtractObservables(pat)
		if !ok {
			continue
		}
		for _, o := range obs {
			k := o.Key()
			byKey[k] = append(byKey[k], n)
		}
	}

	var out []Match
	for _, c := range caseObs {
		inds, hit := byKey[c.Key()]
		if !hit {
			continue
		}
		m := Match{Observable: c.Observable, Context: c.Context}
		seenInd := map[string]bool{}
		seenMal := map[string]bool{}
		seenCamp := map[string]bool{}
		seenActor := map[string]bool{}
		seenTTP := map[string]bool{}
		for _, ind := range inds {
			addRef(&m.Indicators, seenInd, ref(ind))
		}
		for _, ind := range inds {
			for _, e := range s.GetEdgesFrom(ind.ID) {
				if e.Kind != cti.EdgeIndicates {
					continue
				}
				mal, ok := s.GetNode(e.To)
				if !ok {
					continue
				}
				addRef(&m.Malware, seenMal, ref(mal))
				for _, e2 := range s.GetEdgesFrom(mal.ID) {
					if e2.Kind != cti.EdgeUses {
						continue
					}
					if ttp, ok := s.GetNode(e2.To); ok {
						addRef(&m.TTPs, seenTTP, ref(ttp))
					}
				}
				for _, e3 := range s.GetEdgesTo(mal.ID) {
					if e3.Kind != cti.EdgeUses {
						continue
					}
					// STIX models campaign→uses→malware: incoming direction.
					camp, ok := s.GetNode(e3.From)
					if !ok {
						continue
					}
					addRef(&m.Campaigns, seenCamp, ref(camp))
					for _, e4 := range s.GetEdgesFrom(camp.ID) {
						if e4.Kind != cti.EdgeAttributedTo {
							continue
						}
						if actor, ok := s.GetNode(e4.To); ok {
							addRef(&m.Actors, seenActor, ref(actor))
						}
					}
				}
			}
		}
		out = append(out, m)
	}
	return out
}

func ref(n *types.Node) NodeRef {
	name, _ := n.Props["name"].(string)
	sid, _ := n.Props[cti.StixID].(string)
	return NodeRef{ID: n.ID, Label: n.Label, Name: name, StixID: sid}
}

func addRef(list *[]NodeRef, seen map[string]bool, r NodeRef) {
	if seen[r.ID] {
		return
	}
	seen[r.ID] = true
	*list = append(*list, r)
}
