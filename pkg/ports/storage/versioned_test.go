// version: tests — runtime check that inmem implements VersionedStorage.
package storage_test

import (
	"testing"

	"github.com/aprksy/knitknot/pkg/ports/storage"
	"github.com/aprksy/knitknot/pkg/storage/inmem"
)

func TestInmemImplementsVersionedStorage(t *testing.T) {
	if _, ok := any(inmem.New()).(storage.VersionedStorage); !ok {
		t.Fatal("*inmem.Storage does not implement storage.VersionedStorage")
	}
}
