package types

import "time"

// version: types — append-only versioning primitives (ADR 0002, node-only lane).

// Snapshot is a full property capture of one element at one revision.
// History on a Node is append-only with newest-last ordering.
type Snapshot struct {
	Rev         int64          `json:"rev"`
	EventTime   time.Time      `json:"eventTime"`
	LogicalTime time.Time      `json:"logicalTime"`
	Props       map[string]any `json:"props"`
	Source      string         `json:"source"`
	Deleted     bool           `json:"deleted"`
	Transaction string         `json:"transaction"`
}

// Event records one mutation in the per-Storage event log, ordered by Rev.
// ElementType is "node" only in this PR; From/To/Kind stay for future edge events.
type Event struct {
	Rev         int64          `json:"rev"`
	EventTime   time.Time      `json:"eventTime"`
	LogicalTime time.Time      `json:"logicalTime"`
	Op          string         `json:"op"`
	ElementType string         `json:"elementType"`
	ElementID   string         `json:"elementID"`
	From        string         `json:"from"`
	To          string         `json:"to"`
	Kind        string         `json:"kind"`
	Props       map[string]any `json:"props"`
	Source      string         `json:"source"`
	Transaction string         `json:"transaction"`
}

// Node and Edge remain concrete types
type Node struct {
	ID        string               `json:"id"`
	Label     string               `json:"label"`
	Props     map[string]any       `json:"props"`
	Subgraphs map[string]*Subgraph `json:"subgraphs,omitempty"`
	// version: node-history — append-only snapshots, newest last; len >= 1.
	History    []Snapshot `json:"history"`
	DeletedAt  *time.Time `json:"deletedAt,omitempty"`
	CreatedRev int64      `json:"createdRev"`
}

type Edge struct {
	ID        string               `json:"id"`
	From      string               `json:"from"`
	To        string               `json:"to"`
	Kind      string               `json:"kind"`
	Props     map[string]any       `json:"props"`
	Subgraphs map[string]*Subgraph `json:"subgraphs,omitempty"`
	// version: edge — append-only snapshots, newest last; len >= 1.
	History    []Snapshot `json:"history"`
	DeletedAt  *time.Time `json:"deletedAt,omitempty"`
	CreatedRev int64      `json:"createdRev"`
}

type Subgraph struct {
	Name        string
	Description string
}
