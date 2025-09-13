package inmem

import (
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aprksy/knitknot/pkg/ports/types"
	"github.com/aprksy/knitknot/pkg/storage/file"
)

// Save writes the current graph state to disk
func (s *Storage) Save(filename string) error {
	// Ensure dir exists
	_ = os.MkdirAll(filepath.Dir(filename), 0755)

	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	saved := &file.SavedGraph{
		Version: file.CurrentVersion,
		Nodes:   make(map[string]*types.Node),
		Edges:   make(map[string]*types.Edge),
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

	encoder := gob.NewEncoder(f)
	return encoder.Encode(saved)
}

// Load populates the storage from a file
func (s *Storage) Load(filename string) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	var saved file.SavedGraph
	decoder := gob.NewDecoder(f)
	if err := decoder.Decode(&saved); err != nil {
		return err
	}

	if saved.Version != file.CurrentVersion {
		return fmt.Errorf("unsupported version: %s (expected %s)", saved.Version, file.CurrentVersion)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Clear existing
	s.nodes = make(map[string]*types.Node)
	s.edges = make(map[string]*types.Edge)

	// Restore
	for id, n := range saved.Nodes {
		s.nodes[id] = n
	}
	for id, e := range saved.Edges {
		s.edges[id] = e
	}

	return nil
}
