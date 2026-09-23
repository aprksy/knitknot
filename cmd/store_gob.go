package cmd

import (
	"fmt"
	"strings"

	portstorage "github.com/aprksy/knitknot/pkg/ports/storage"
	"github.com/aprksy/knitknot/pkg/storage/inmem"
)

// store: resolver — ADR 0001 selection rule: --store wins, else -f is
// gob:<path>, else ephemeral mem.
func ResolveStoreURI(storeFlag, fileFlag string) (scheme, path string, err error) {
	if storeFlag != "" {
		idx := strings.Index(storeFlag, ":")
		if idx <= 0 {
			return "", "", fmt.Errorf("invalid --store %q: want <scheme>:<path>", storeFlag)
		}
		return storeFlag[:idx], storeFlag[idx+1:], nil
	}
	if fileFlag != "" {
		return "gob", fileFlag, nil
	}
	return "mem", "", nil
}

// store: backend — thin gob wrapper around inmem; backends don't auto-save.
type gobBackend struct{}

func (gobBackend) Kind() string { return "gob" }

func (gobBackend) Engine() portstorage.StorageEngine { return inmem.New() }

func (gobBackend) Create(path string) (portstorage.StorageEngine, error) {
	return inmem.New(), nil // don't write yet
}

func (gobBackend) Open(path string) (portstorage.StorageEngine, error) {
	engine, err := LoadGraph(path) // LoadGraph covers "" + missing file
	if err != nil {
		return nil, err
	}
	return engine.Storage(), nil
}
