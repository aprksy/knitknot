package file

import (
	"bytes"
	"encoding/gob"
	"reflect"
	"testing"
	"time"

	"github.com/aprksy/knitknot/pkg/ports/types"
)

func init() {
	// Same concrete registrations the gob consumer
	// (pkg/storage/inmem/persist.go) relies on for JSON-shaped prop values.
	gob.Register([]interface{}{})
	gob.Register(map[string]interface{}{})
}

func TestSavedGraphRoundTrip(t *testing.T) {
	now := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name  string
		saved SavedGraph
	}{
		{
			name:  "empty",
			saved: SavedGraph{Version: CurrentVersion},
		},
		{
			name: "nodes and edges",
			saved: SavedGraph{
				Version: CurrentVersion,
				Nodes: map[string]*types.Node{
					"n1": {
						ID:    "n1",
						Label: "User",
						Props: map[string]any{"name": "Alice", "age": 30},
						History: []types.Snapshot{{
							Rev:         1,
							LogicalTime: now,
							EventTime:   now,
							Props:       map[string]any{"name": "Alice", "age": 30},
							Source:      "test",
						}},
						CreatedRev: 1,
					},
				},
				Edges: map[string]*types.Edge{
					"n1->n2@knows": {
						ID:    "n1->n2@knows",
						From:  "n1",
						To:    "n2",
						Kind:  "knows",
						Props: map[string]any{"since": "2023-01-01", "level": 4, "verified": true},
						History: []types.Snapshot{{
							Rev:         2,
							LogicalTime: now,
							EventTime:   now,
							Props:       map[string]any{"since": "2023-01-01"},
							Source:      "test",
						}},
						CreatedRev: 2,
					},
				},
				Verbs: map[string]types.Verb{
					"knows": {MatchOn: "name"},
				},
			},
		},
		{
			name: "full with rev counter event log and nested props",
			saved: SavedGraph{
				Version: CurrentVersion,
				Nodes: map[string]*types.Node{
					"n1": {
						ID:    "n1",
						Label: "User",
						Props: map[string]any{
							"name": "Alice",
							"tags": []any{"admin", "owner"},
							"meta": map[string]any{"team": "core"},
						},
						History: []types.Snapshot{{
							Rev:         3,
							LogicalTime: now,
							EventTime:   now,
							Props:       map[string]any{"name": "Alice"},
							Source:      "test",
						}},
						DeletedAt:  &now,
						CreatedRev: 1,
					},
				},
				Edges:      map[string]*types.Edge{},
				RevCounter: 3,
				EventLog: []types.Event{
					{
						Rev:         3,
						LogicalTime: now,
						EventTime:   now,
						Op:          "delete",
						ElementType: "node",
						ElementID:   "n1",
						Props:       map[string]any{"name": "Alice"},
						Source:      "test",
					},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := gob.NewEncoder(&buf).Encode(&tc.saved); err != nil {
				t.Fatalf("encode: %v", err)
			}
			var got SavedGraph
			if err := gob.NewDecoder(&buf).Decode(&got); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if got.Version != CurrentVersion {
				t.Errorf("version = %q, want %q", got.Version, CurrentVersion)
			}
			if !reflect.DeepEqual(&tc.saved, &got) {
				t.Errorf("round-trip mismatch:\n got: %#v\nwant: %#v", &got, &tc.saved)
			}
		})
	}
}
