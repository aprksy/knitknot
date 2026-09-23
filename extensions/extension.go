// Package extension defines the minimal contract for compiled-in
// domain extensions. See docs/domains/README.md for the rules.
package extension

import (
	"context"
	"errors"
	"io"
	"sync"

	"github.com/aprksy/knitknot/pkg/ports/storage"
	"github.com/aprksy/knitknot/pkg/ports/types"
)

type Extension interface {
	Name() string
	Register(Registry) error
}

// ImportContext carries feed metadata for an import run. // ext: source-feed
type ImportContext struct { // ext: source-feed
	Source      string // ext: source-feed — feed id, set by `knitknot import --source <feed-id>`
	Transaction string // ext: source-feed — optional override; empty = auto-UUID
} // ext: source-feed

type Importer interface {
	Import(ctx context.Context, ic ImportContext, s storage.StorageEngine, verbs *types.VerbRegistry, r io.Reader) error
}

type Exporter interface {
	Export(ctx context.Context, s storage.StorageEngine, verbs *types.VerbRegistry, w io.Writer) error
}

type Registry interface {
	RegisterVerb(name string, verb types.Verb) error
	RegisterImporter(name string, importer Importer) error
	RegisterExporter(name string, exporter Exporter) error
	Importer(name string) (Importer, bool)
	Exporter(name string) (Exporter, bool)
	Freeze()
}

type registry struct { // ext: registry
	mu        sync.RWMutex          // ext: registry
	verbs     map[string]types.Verb // ext: registry
	importers map[string]Importer   // ext: registry
	exporters map[string]Exporter   // ext: registry
	frozen    bool                  // ext: registry
}

func NewRegistry() Registry { // ext: registry
	return &registry{ // ext: registry
		verbs:     make(map[string]types.Verb), // ext: registry
		importers: make(map[string]Importer),   // ext: registry
		exporters: make(map[string]Exporter),   // ext: registry
	}
}

func (r *registry) RegisterVerb(name string, verb types.Verb) error {
	r.mu.Lock() // ext: registry
	defer r.mu.Unlock()
	if r.frozen { // ext: registry
		return errors.New("registry is frozen") // ext: registry
	}
	if name == "" {
		return errors.New("verb name must not be empty")
	}
	if _, ok := r.verbs[name]; ok {
		return errors.New("duplicate verb: " + name)
	}
	r.verbs[name] = verb // ext: registry
	return nil
}

func (r *registry) RegisterImporter(name string, importer Importer) error {
	r.mu.Lock() // ext: registry
	defer r.mu.Unlock()
	if r.frozen { // ext: registry
		return errors.New("registry is frozen") // ext: registry
	}
	if name == "" {
		return errors.New("importer name must not be empty")
	}
	if importer == nil {
		return errors.New("importer must not be nil")
	}
	if _, ok := r.importers[name]; ok {
		return errors.New("duplicate importer: " + name)
	}
	r.importers[name] = importer // ext: registry
	return nil
}

func (r *registry) RegisterExporter(name string, exporter Exporter) error {
	r.mu.Lock() // ext: registry
	defer r.mu.Unlock()
	if r.frozen { // ext: registry
		return errors.New("registry is frozen") // ext: registry
	}
	if name == "" {
		return errors.New("exporter name must not be empty")
	}
	if exporter == nil {
		return errors.New("exporter must not be nil")
	}
	if _, ok := r.exporters[name]; ok {
		return errors.New("duplicate exporter: " + name)
	}
	r.exporters[name] = exporter // ext: registry
	return nil
}

func (r *registry) Importer(name string) (Importer, bool) {
	r.mu.RLock() // ext: registry
	defer r.mu.RUnlock()
	imp, ok := r.importers[name] // ext: registry
	if !ok {
		return nil, false
	}
	return imp, true
}

func (r *registry) Exporter(name string) (Exporter, bool) {
	r.mu.RLock() // ext: registry
	defer r.mu.RUnlock()
	exp, ok := r.exporters[name] // ext: registry
	if !ok {
		return nil, false
	}
	return exp, true
}

func (r *registry) Freeze() {
	r.mu.Lock() // ext: registry
	defer r.mu.Unlock()
	r.frozen = true // ext: registry
}
