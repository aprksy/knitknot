package inmem

import (
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/aprksy/knitknot/pkg/graph"
	"github.com/aprksy/knitknot/pkg/ports/types"
	"github.com/aprksy/knitknot/pkg/storage/file"
)

func init() {
	gob.Register(map[string]*types.Subgraph{}) // fix: H6
}

// Save writes the current graph state to disk
func (s *Storage) Save(filename string, engine *graph.GraphEngine) (err error) {
	// Ensure dir exists
	_ = os.MkdirAll(filepath.Dir(filename), 0755)

	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	saved := &file.SavedGraph{
		Version: file.CurrentVersion,
		Nodes:   make(map[string]*types.Node),
		Edges:   make(map[string]*types.Edge),
		Verbs:   make(map[string]types.Verb),
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	// Copy nodes and edges
	for id, n := range s.nodes {
		saved.Nodes[id] = n
	}
	for id, e := range s.edges {
		saved.Edges[id] = e
	}
	// version: persist — event log + rev counter ride along top-level.
	saved.RevCounter = s.revCounter.Load()
	saved.EventLog = append([]types.Event(nil), s.eventLog...)

	if engine != nil {
		saved.Verbs = engine.Verbs().All()
	}

	encoder := gob.NewEncoder(f)
	return encoder.Encode(saved)
}

// Load populates the storage from a file
func (s *Storage) Load(filename string, engine *graph.GraphEngine) (err error) {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	var saved file.SavedGraph
	decoder := gob.NewDecoder(f)
	if err := decoder.Decode(&saved); err != nil {
		return fmt.Errorf("failed to decode %s (expected version %s): %w", filename, file.CurrentVersion, err) // fix: H6
	}

	// version: migrate — v0.3 and earlier load with synthesized history; anything else must match.
	switch saved.Version {
	case file.CurrentVersion, "knitknot/v0.3", "knitknot/v0.2":
	default:
		return fmt.Errorf("unsupported version: %s (expected %s)", saved.Version, file.CurrentVersion)
	}

	if s.now == nil {
		s.now = time.Now
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Clear existing
	s.nodes = make(map[string]*types.Node)
	s.edges = make(map[string]*types.Edge)
	s.nodesByLabel = make(map[string]map[string]*types.Node) // perf: index

	// Restore
	for id, n := range saved.Nodes {
		if len(n.History) == 0 {
			// version: migrate — legacy nodes get one honest synthetic
			// snapshot: Rev=1, unknown times, Source="legacy".
			n.History = []types.Snapshot{{
				Rev:         1,
				LogicalTime: time.Time{},
				EventTime:   time.Time{},
				Props:       copyMap(n.Props),
				Source:      "legacy",
			}}
			n.CreatedRev = 1
		}
		s.nodes[id] = n
		if s.nodesByLabel[n.Label] == nil { // perf: index
			s.nodesByLabel[n.Label] = make(map[string]*types.Node) // perf: index
		} // perf: index
		s.nodesByLabel[n.Label][id] = n // perf: index
	}
	for id, e := range saved.Edges {
		if len(e.History) == 0 {
			// version: edge — legacy edges get one honest synthetic
			// snapshot: Rev=1, unknown times, Source="legacy".
			e.History = []types.Snapshot{{
				Rev:         1,
				LogicalTime: time.Time{},
				EventTime:   time.Time{},
				Props:       copyMap(e.Props),
				Source:      "legacy",
			}}
			e.CreatedRev = 1
		}
		s.edges[id] = e
	}

	// version: persist — restore the log; rev counter resumes past the
	// highest rev seen so no revision is ever reused.
	s.eventLog = append([]types.Event(nil), saved.EventLog...)
	maxRev := saved.RevCounter
	for _, n := range s.nodes {
		for _, snap := range n.History {
			if uint64(snap.Rev) > maxRev {
				maxRev = uint64(snap.Rev)
			}
		}
	}
	// version: edge — edge snapshots also advance the counter.
	for _, e := range s.edges {
		for _, snap := range e.History {
			if uint64(snap.Rev) > maxRev {
				maxRev = uint64(snap.Rev)
			}
		}
	}
	for _, e := range s.eventLog {
		if uint64(e.Rev) > maxRev {
			maxRev = uint64(e.Rev)
		}
	}
	s.revCounter.Store(maxRev)

	// After restoring nodes/edges
	if engine != nil {
		for name, verb := range saved.Verbs {
			engine.RegisterVerb(name, verb) // assuming engine is passed in
		}
	}

	return nil
}
