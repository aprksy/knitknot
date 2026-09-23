package cmd

import (
	"github.com/aprksy/knitknot/pkg/graph"
	portstorage "github.com/aprksy/knitknot/pkg/ports/storage"
)

// store: wire — resolveStoreBackend returns the engine to use. If --store is
// set and non-empty, open the backend through the selector. Otherwise the
// existing engine (from LoadGraph) is used unchanged.
func resolveStoreBackend(engine *graph.GraphEngine) (*graph.GraphEngine, error) {
	if storeFlag == "" {
		return engine, nil
	}
	ensureDefaultStore() // store: wire — idempotent default registration
	scheme, path, err := ResolveStoreURI(storeFlag, "")
	if err != nil {
		return nil, err
	}
	backend, err := portstorage.Default().Resolve(scheme + ":" + path) // store: wire
	if err != nil {
		return nil, err
	}
	var se portstorage.StorageEngine
	if path == "" {
		se = backend.Engine() // store: wire — no path, ephemeral engine
	} else {
		se, err = backend.Open(path) // store: wire — open via backend
		if err != nil {
			return nil, err
		}
	}
	return graph.NewGraphEngine(se), nil
}
