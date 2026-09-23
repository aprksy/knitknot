// version: storage-contract — optional VersionedStorage (ADR 0002, node-only lane).
package storage

import (
	"time"

	"github.com/aprksy/knitknot/pkg/ports/types"
)

// VersionedStorage is an optional interface alongside StorageEngine.
// Backends that support append-only versioning implement it; backends
// that don't simply don't (callers fall back to StorageEngine).
// Edge history is deferred to a follow-up; this lane is nodes only.
type VersionedStorage interface {
	StorageEngine

	// GetNodeAt returns the element as of time t: the latest snapshot
	// with EventTime <= t. A tombstone at t (or zero t / unknown id)
	// returns not-found. Use GetNode for the current view.
	GetNodeAt(id string, t time.Time) (*types.Node, bool)

	// GetNodeHistory returns the element's full history, newest-last.
	GetNodeHistory(id string) []types.Snapshot

	// GetEvents returns events in [fromRev, toRev] inclusive, Rev ascending.
	// toRev == 0 means "to current rev".
	GetEvents(fromRev, toRev int64) []types.Event

	// GetEventsByTransaction returns events sharing a transaction ID, Rev order.
	GetEventsByTransaction(txID string) []types.Event

	// EventsTouching returns events for one element ID, Rev order.
	EventsTouching(elementID string) []types.Event
}
