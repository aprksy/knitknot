package dfir

import (
	"fmt"
	"sort"
	"strings"

	"github.com/aprksy/knitknot/pkg/ports/storage"
	"github.com/aprksy/knitknot/pkg/ports/types"
)

// Coverage labels and edge kinds beyond vocab.go. Declared locally because
// docs/domains/README.md forbids a domain extension importing another
// domain extension (see vocab.go).
const (
	LabelAttackPattern = "attack-pattern"

	EdgeCommunicatesWith = "communicates-with"
	EdgeTargets          = "targets"
	EdgeHasTactic        = "has-tactic"
	EdgeDerivedFrom      = "derived-from"
	EdgeObjectRef        = "object-ref"
)

// DefaultCoverageDepth is the BFS radius when maxDepth <= 0: enough to reach
// indicators/techniques from an actor through the standard chain
// (actor—campaign—malware—indicator/technique).
const DefaultCoverageDepth = 3

// CoverageDimensions are the directly-evidenced labels v1 reports, in
// stable order.
var CoverageDimensions = []string{LabelIndicator, LabelAttackPattern}

// coverageEdgeKinds are the CTI edge kinds traversed for pattern extraction
// and inferred context.
var coverageEdgeKinds = map[string]bool{
	EdgeUses:             true,
	EdgeIndicates:        true,
	EdgeAttributedTo:     true,
	EdgeBasedOn:          true,
	EdgeCommunicatesWith: true,
	EdgeTargets:          true,
	EdgeHasTactic:        true,
	EdgeDerivedFrom:      true,
	EdgeObjectRef:        true,
}

// DimensionCoverage is one label's coverage fraction: Covered of Total
// pattern nodes directly evidenced, with the matched/unmatched members.
type DimensionCoverage struct {
	Label     string    `json:"label"`
	Covered   int       `json:"covered"`
	Total     int       `json:"total"`
	Matched   []NodeRef `json:"matched"`
	Unmatched []NodeRef `json:"unmatched"`
}

// CoverageReport is the coverage of one reference target by case evidence.
// Inferred is CTI context reachable from the matched indicators — reported
// separately, never counted. A Total of 0 means the dimension is absent
// from the pattern (n/a), not 100%.
type CoverageReport struct {
	Target     NodeRef             `json:"target"`
	Dimensions []DimensionCoverage `json:"dimensions"`
	Inferred   []NodeRef           `json:"inferred"`
}

// PatternOf returns the target's CTI neighbourhood grouped by node label:
// a BFS from the target in both directions over the CTI edge kinds, up to
// maxDepth (<=0 defaults to DefaultCoverageDepth). The target itself is
// excluded. Deduped; never mutates storage.
func PatternOf(s storage.StorageEngine, target *types.Node, maxDepth int) map[string][]*types.Node {
	if maxDepth <= 0 {
		maxDepth = DefaultCoverageDepth
	}
	visited := bfsNodes(s, []*types.Node{target}, maxDepth)
	out := make(map[string][]*types.Node)
	for id, n := range visited {
		if id == target.ID {
			continue
		}
		out[n.Label] = append(out[n.Label], n)
	}
	for _, nodes := range out {
		sortCoverageNodes(nodes)
	}
	return out
}

// DirectEvidence maps case entities to the CTI nodes they directly evidence,
// grouped by node label: observables to indicators (canonical
// pattern-observable equality via the same index Correlate uses) and
// techniques to attack-patterns (case-insensitive name equality).
// Deduped; never mutates storage.
func DirectEvidence(s storage.StorageEngine, entities []CaseEntity) map[string][]*types.Node {
	out := make(map[string][]*types.Node)
	seen := make(map[string]map[string]bool)
	add := func(n *types.Node) {
		m := seen[n.Label]
		if m == nil {
			m = make(map[string]bool)
			seen[n.Label] = m
		}
		if m[n.ID] {
			return
		}
		m[n.ID] = true
		out[n.Label] = append(out[n.Label], n)
	}

	var obs []CaseEntity
	var techs []CaseEntity
	for _, e := range entities {
		if e.Kind == EntityTechnique {
			techs = append(techs, e)
		} else {
			obs = append(obs, e)
		}
	}

	if len(obs) > 0 {
		byKey := indicatorIndex(s)
		for _, e := range obs {
			for _, n := range byKey[Observable{Type: e.Kind, Value: e.Value}.Key()] {
				add(n)
			}
		}
	}
	if len(techs) > 0 {
		for _, n := range s.GetNodesByLabel(LabelAttackPattern) {
			name, _ := n.Props["name"].(string)
			if name == "" {
				continue
			}
			for _, e := range techs {
				if strings.EqualFold(name, e.Value) {
					add(n)
					break
				}
			}
		}
	}
	for _, nodes := range out {
		sortCoverageNodes(nodes)
	}
	return out
}

// Coverage measures how much of the target's pattern the case entities
// directly evidence, per CoverageDimensions. Covered is the intersection
// size of pattern and direct-evidence nodes for the label; Total is the
// pattern size (0 = dimension absent, n/a). Inferred is the CTI context
// reachable from the matched indicators minus the directly-evidenced nodes
// and the target itself. All lists sort by name for testability.
func Coverage(s storage.StorageEngine, target *types.Node, entities []CaseEntity, maxDepth int) (CoverageReport, error) {
	if target == nil {
		return CoverageReport{}, fmt.Errorf("dfir: coverage: nil target")
	}
	if maxDepth <= 0 {
		maxDepth = DefaultCoverageDepth
	}
	pattern := PatternOf(s, target, maxDepth)
	direct := DirectEvidence(s, entities)

	directIDs := make(map[string]bool)
	for _, nodes := range direct {
		for _, n := range nodes {
			directIDs[n.ID] = true
		}
	}

	rep := CoverageReport{Target: ref(target), Inferred: []NodeRef{}}
	for _, dim := range CoverageDimensions {
		dc := DimensionCoverage{Label: dim, Matched: []NodeRef{}, Unmatched: []NodeRef{}}
		dirIDs := make(map[string]bool)
		for _, n := range direct[dim] {
			dirIDs[n.ID] = true
		}
		for _, n := range pattern[dim] {
			if dirIDs[n.ID] {
				dc.Matched = append(dc.Matched, ref(n))
			} else {
				dc.Unmatched = append(dc.Unmatched, ref(n))
			}
		}
		dc.Covered = len(dc.Matched)
		dc.Total = len(pattern[dim])
		sortNodeRefs(dc.Matched)
		sortNodeRefs(dc.Unmatched)
		rep.Dimensions = append(rep.Dimensions, dc)
	}

	for _, n := range bfsNodes(s, direct[LabelIndicator], maxDepth) {
		if n.ID == target.ID || directIDs[n.ID] {
			continue
		}
		rep.Inferred = append(rep.Inferred, ref(n))
	}
	sortNodeRefs(rep.Inferred)
	return rep, nil
}

// ResolveTarget finds the single node with the given label and name prop.
// It errors when no node or more than one matches.
func ResolveTarget(s storage.StorageEngine, label, name string) (*types.Node, error) {
	var found []*types.Node
	for _, n := range s.GetNodesByLabel(label) {
		if nm, _ := n.Props["name"].(string); nm == name {
			found = append(found, n)
		}
	}
	switch len(found) {
	case 0:
		return nil, fmt.Errorf("dfir: coverage: no node with label %q and name %q", label, name)
	case 1:
		return found[0], nil
	default:
		return nil, fmt.Errorf("dfir: coverage: ambiguous target %q:%q: %d nodes match", label, name, len(found))
	}
}

// indicatorIndex maps Observable.Key() to every indicator whose pattern
// carries that observable; indicators with unsupported patterns are
// skipped. Shared by Correlate and DirectEvidence.
func indicatorIndex(s storage.StorageEngine) map[string][]*types.Node {
	byKey := make(map[string][]*types.Node)
	for _, n := range s.GetNodesByLabel(LabelIndicator) {
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
	return byKey
}

// bfsNodes walks both directions over the CTI edge kinds from every start
// node, up to maxDepth hops. The returned set includes the starts.
func bfsNodes(s storage.StorageEngine, starts []*types.Node, maxDepth int) map[string]*types.Node {
	visited := make(map[string]*types.Node)
	depth := make(map[string]int)
	var queue []*types.Node
	for _, n := range starts {
		if n == nil {
			continue
		}
		if _, ok := visited[n.ID]; !ok {
			visited[n.ID] = n
			depth[n.ID] = 0
			queue = append(queue, n)
		}
	}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if depth[cur.ID] >= maxDepth {
			continue
		}
		for _, nb := range coverageNeighbors(s, cur.ID) {
			if _, ok := visited[nb.ID]; ok {
				continue
			}
			visited[nb.ID] = nb
			depth[nb.ID] = depth[cur.ID] + 1
			queue = append(queue, nb)
		}
	}
	return visited
}

// coverageNeighbors returns adjacent nodes over CTI edge kinds in both
// directions, deduped.
func coverageNeighbors(s storage.StorageEngine, id string) []*types.Node {
	var out []*types.Node
	seen := make(map[string]bool)
	add := func(n *types.Node) {
		if !seen[n.ID] {
			seen[n.ID] = true
			out = append(out, n)
		}
	}
	for _, e := range s.GetEdgesFrom(id) {
		if !coverageEdgeKinds[e.Kind] {
			continue
		}
		if n, ok := s.GetNode(e.To); ok {
			add(n)
		}
	}
	for _, e := range s.GetEdgesTo(id) {
		if !coverageEdgeKinds[e.Kind] {
			continue
		}
		if n, ok := s.GetNode(e.From); ok {
			add(n)
		}
	}
	return out
}

func sortCoverageNodes(nodes []*types.Node) {
	sort.Slice(nodes, func(i, j int) bool {
		ni, _ := nodes[i].Props["name"].(string)
		nj, _ := nodes[j].Props["name"].(string)
		if ni != nj {
			return ni < nj
		}
		return nodes[i].ID < nodes[j].ID
	})
}

func sortNodeRefs(refs []NodeRef) {
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].Name != refs[j].Name {
			return refs[i].Name < refs[j].Name
		}
		return refs[i].ID < refs[j].ID
	})
}
