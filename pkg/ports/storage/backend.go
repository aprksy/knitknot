// store: backend — ADR 0001 pluggable storage backend contract.
package storage

import (
	"fmt"
	"strings"
	"sync"
)

// Backend is a storage implementation selectable by URI scheme.
type Backend interface {
	Kind() string                              // "gob", "sqlite", ...
	Open(path string) (StorageEngine, error)   // open existing file
	Create(path string) (StorageEngine, error) // create new file
	Engine() StorageEngine                     // in-process / no file
}

// Selector maps store URIs to registered backends.
type Selector interface {
	Register(b Backend)
	Resolve(uri string) (Backend, error)
}

type defaultSelector struct {
	mu     sync.RWMutex
	byKind map[string]Backend
}

func (s *defaultSelector) Register(b Backend) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.byKind[b.Kind()]; ok {
		return // idempotent: re-register of the same scheme is a no-op
	}
	s.byKind[b.Kind()] = b
}

func (s *defaultSelector) Resolve(uri string) (Backend, error) {
	idx := strings.Index(uri, ":")
	if idx <= 0 {
		return nil, fmt.Errorf("invalid store URI %q: missing scheme (want <scheme>:<path>)", uri)
	}
	scheme := uri[:idx]
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, ok := s.byKind[scheme]
	if !ok {
		return nil, fmt.Errorf("unknown store scheme %q", scheme)
	}
	return b, nil
}

var defaultSel = &defaultSelector{byKind: map[string]Backend{}}

// Default returns the process-singleton selector.
func Default() Selector { return defaultSel }

// Register adds a backend to the process-singleton selector.
func Register(b Backend) { Default().Register(b) }
