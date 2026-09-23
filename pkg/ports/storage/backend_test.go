// store: backend — contract tests for the ADR 0001 selector.
package storage_test

import (
	"fmt"
	"sync"
	"testing"

	portstorage "github.com/aprksy/knitknot/pkg/ports/storage"
	"github.com/aprksy/knitknot/pkg/storage/inmem"
)

type stubBackend struct{ kind string }

func (s *stubBackend) Kind() string { return s.kind }
func (s *stubBackend) Open(path string) (portstorage.StorageEngine, error) {
	return inmem.New(), nil
}
func (s *stubBackend) Create(path string) (portstorage.StorageEngine, error) {
	return inmem.New(), nil
}
func (s *stubBackend) Engine() portstorage.StorageEngine { return inmem.New() }

func TestResolveRegisteredScheme(t *testing.T) {
	portstorage.Register(&stubBackend{kind: "stubtest-resolve"})
	b, err := portstorage.Default().Resolve("stubtest-resolve:any/path.gob")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if b.Kind() != "stubtest-resolve" {
		t.Fatalf("Kind = %q, want %q", b.Kind(), "stubtest-resolve")
	}
}

func TestResolveUnknownSchemeErrors(t *testing.T) {
	if _, err := portstorage.Default().Resolve("nosuchbackend-xyz:file.gob"); err == nil {
		t.Fatal("expected error for unknown scheme, got nil")
	}
}

func TestResolveMissingSchemeErrors(t *testing.T) {
	for _, uri := range []string{"", "noscheme", ":path"} {
		if _, err := portstorage.Default().Resolve(uri); err == nil {
			t.Fatalf("expected parse error for %q, got nil", uri)
		}
	}
}

func TestReRegisterSameSchemeIsNoop(t *testing.T) {
	first := &stubBackend{kind: "stubtest-rereg"}
	portstorage.Register(first)
	portstorage.Register(&stubBackend{kind: "stubtest-rereg"})
	b, err := portstorage.Default().Resolve("stubtest-rereg:x")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if b != first {
		t.Fatal("re-register replaced the original backend; want no-op")
	}
}

func TestConcurrentRegister(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			portstorage.Register(&stubBackend{kind: fmt.Sprintf("stubtest-conc-%d", i%4)})
		}(i)
	}
	wg.Wait()
	for i := 0; i < 4; i++ {
		if _, err := portstorage.Default().Resolve(fmt.Sprintf("stubtest-conc-%d:x", i)); err != nil {
			t.Fatalf("Resolve: %v", err)
		}
	}
}
