package extension_test

import (
	"context"
	"io"
	"strings"
	"testing"

	extension "github.com/aprksy/knitknot/extensions"
	"github.com/aprksy/knitknot/pkg/ports/storage"
	"github.com/aprksy/knitknot/pkg/ports/types"
)

type stubImporter struct{} // ext: registry

func (stubImporter) Import(ctx context.Context, s storage.StorageEngine, verbs *types.VerbRegistry, r io.Reader) error { // ext: registry
	return nil
}

type stubExporter struct{} // ext: registry

func (stubExporter) Export(ctx context.Context, s storage.StorageEngine, verbs *types.VerbRegistry, w io.Writer) error { // ext: registry
	return nil
}

func TestRegisterThenFreezeLookupsSucceed(t *testing.T) { // ext: registry
	reg := extension.NewRegistry()
	if err := reg.RegisterVerb("rel", types.Verb{}); err != nil {
		t.Fatalf("RegisterVerb: %v", err)
	}
	if err := reg.RegisterImporter("imp", stubImporter{}); err != nil {
		t.Fatalf("RegisterImporter: %v", err)
	}
	if err := reg.RegisterExporter("exp", stubExporter{}); err != nil {
		t.Fatalf("RegisterExporter: %v", err)
	}
	reg.Freeze()
	if _, ok := reg.Importer("imp"); !ok {
		t.Fatal("Importer lookup failed after Freeze")
	}
	if _, ok := reg.Exporter("exp"); !ok {
		t.Fatal("Exporter lookup failed after Freeze")
	}
}

func TestDuplicateNameError(t *testing.T) { // ext: registry
	reg := extension.NewRegistry()
	if err := reg.RegisterVerb("v", types.Verb{}); err != nil {
		t.Fatalf("first RegisterVerb: %v", err)
	}
	if err := reg.RegisterVerb("v", types.Verb{}); err == nil {
		t.Fatal("second RegisterVerb should error")
	}
	if err := reg.RegisterImporter("i", stubImporter{}); err != nil {
		t.Fatalf("first RegisterImporter: %v", err)
	}
	if err := reg.RegisterImporter("i", stubImporter{}); err == nil {
		t.Fatal("second RegisterImporter should error")
	}
	if err := reg.RegisterExporter("e", stubExporter{}); err != nil {
		t.Fatalf("first RegisterExporter: %v", err)
	}
	if err := reg.RegisterExporter("e", stubExporter{}); err == nil {
		t.Fatal("second RegisterExporter should error")
	}
}

func TestRegisterAfterFreezeError(t *testing.T) { // ext: registry
	reg := extension.NewRegistry()
	reg.Freeze()
	for _, err := range []error{
		reg.RegisterVerb("v", types.Verb{}),
		reg.RegisterImporter("i", stubImporter{}),
		reg.RegisterExporter("e", stubExporter{}),
	} {
		if err == nil {
			t.Fatal("register after Freeze should error")
		}
		if !strings.Contains(err.Error(), "frozen") {
			t.Fatalf("error should mention frozen, got: %v", err)
		}
	}
}

func TestLookupMissing(t *testing.T) { // ext: registry
	reg := extension.NewRegistry()
	if imp, ok := reg.Importer("nope"); ok || imp != nil {
		t.Fatal("missing Importer should be (nil, false)")
	}
	if exp, ok := reg.Exporter("nope"); ok || exp != nil {
		t.Fatal("missing Exporter should be (nil, false)")
	}
}

func TestFreezeTwiceNoPanic(t *testing.T) { // ext: registry
	reg := extension.NewRegistry()
	reg.Freeze()
	reg.Freeze() // second is a no-op
}
