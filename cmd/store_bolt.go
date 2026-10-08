package cmd

import (
	portstorage "github.com/aprksy/knitknot/pkg/ports/storage"
	"github.com/aprksy/knitknot/pkg/storage/bolt"
	"github.com/aprksy/knitknot/pkg/storage/inmem"
)

// store: backend — bolt wrapper around pkg/storage/bolt (ADR 0003, P4).
// bbolt is crash-safe: writes are durable on return, no auto-save needed.
type boltBackend struct{}

func (boltBackend) Kind() string { return "bolt" }

// Engine is process-lifetime bolt on a temp file; falls back to inmem only
// if no temp file can be made (bbolt has no pure in-memory mode).
func (boltBackend) Engine() portstorage.StorageEngine {
	if se, err := bolt.OpenTemp(); err == nil {
		return se
	}
	return inmem.New()
}

func (boltBackend) Create(path string) (portstorage.StorageEngine, error) {
	return bolt.OpenNoSync(path) // bulk imports: skip per-tx fsync
}

func (boltBackend) Open(path string) (portstorage.StorageEngine, error) {
	return bolt.Open(path) // bbolt creates on open; missing file = empty graph
}
