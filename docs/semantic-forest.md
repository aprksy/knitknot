# Technical Specification: AxiomFlow Semantic Forest Subsystem

## 1. Overview
The Semantic Forest is AxiomFlow's cognitive memory subsystem. It manages user-specific episodic memory, transforming chronological conversation logs into a structured, topic-aware semantic forest. 

It is strictly decoupled from the AxiomFlow Petri Net runtime via a **Contract Layer**, driven by a **Tree & Refactoring Engine**, and persisted via pluggable **Backend Adapters**.

**Core Directives for AI Dev Team:**
1. **No Side Channels:** The Engine must NEVER call external LLM or Embedding APIs directly. It must use injected interfaces (`LLMInvoker`, `Embedder`) which the AxiomFlow Kernel will implement via Petri Net syscalls.
2. **Immutability:** `ContextNode` payloads are immutable. Updates create new versions.
3. **Strict Interfaces:** The Engine only interacts with the Adapter via the `MemoryStore` interface.

---

## 2. Data Models (`models.go`)

Define the core domain objects. All objects must be serializable to JSON.

```go
package memory

import "time"

// NodeType defines the semantic role of a node in the tree.
type NodeType string

const (
    NodeTypeUserProfile     NodeType = "user_profile"
    NodeTypeSessionSummary  NodeType = "session_summary" // L1/L2 summary
    NodeTypePRTurn          NodeType = "pr_turn"         // Raw Prompt-Response pair
    NodeTypeTopicCluster    NodeType = "topic_cluster"   // Refactored semantic root
)

// ContextNode is the fundamental unit of the Semantic Forest.
type ContextNode struct {
    ID         string            `json:"id"`
    Type       NodeType          `json:"type"`
    UserID     string            `json:"user_id"`
    ParentID   string            `json:"parent_id,omitempty"`
    ChildrenID []string          `json:"children_ids,omitempty"`
    
    // Payload holds the structured data (e.g., {prompt: "...", response: "..."}).
    // It is NEVER raw text; it is always structured.
    Payload    map[string]any    `json:"payload"`
    
    // Embedding is the vector representation of the Payload.
    Embedding  []float32         `json:"embedding,omitempty"`
    
    Metadata   NodeMetadata      `json:"metadata"`
}

type NodeMetadata struct {
    CreatedAt       time.Time `json:"created_at"`
    SourceTraceID   string    `json:"source_trace_id"` // Links to Petri Net execution
    Version         int       `json:"version"`
    Tags            []string  `json:"tags,omitempty"`
    IsStale         bool      `json:"is_stale"`        // Used for corrections
    SupersededBy    string    `json:"superseded_by,omitempty"`
}

// RetrievalQuery defines how the Kernel Net asks for memory.
type RetrievalQuery struct {
    UserID      string            `json:"user_id"`
    QueryVector []float32         `json:"query_vector,omitempty"`
    TextQuery   string            `json:"text_query,omitempty"` // Fallback/hybrid
    Filters     RetrievalFilters  `json:"filters"`
    Limit       int               `json:"limit"`
}

type RetrievalFilters struct {
    NodeTypes   []NodeType `json:"node_types,omitempty"`
    MaxDepth    int        `json:"max_depth,omitempty"`
    TopicIDs    []string   `json:"topic_ids,omitempty"`
    TimeRange   *TimeRange `json:"time_range,omitempty"`
    ExcludeStale bool      `json:"exclude_stale"`
}

type TimeRange struct {
    Start time.Time `json:"start"`
    End   time.Time `json:"end"`
}
```

---

## 3. Contract Layer (`contract.go`)

This defines the "Ports" (Hexagonal Architecture). The Petri Net talks to the `MemorySubsystem` interface.

```go
package memory

import "context"

// MemorySubsystem is the top-level port exposed to the AxiomFlow Kernel Net.
type MemorySubsystem interface {
    // StoreNodes persists new nodes (e.g., a new PR turn).
    StoreNodes(ctx context.Context, nodes []ContextNode) error
    
    // RetrieveContext fetches relevant memory for LLM context building.
    RetrieveContext(ctx context.Context, query RetrievalQuery) ([]ContextNode, error)
    
    // GetLineage returns the ancestry path from a node to the root.
    GetLineage(ctx context.Context, nodeID string) ([]ContextNode, error)
    
    // TriggerRefactor initiates the offline semantic refactoring process.
    // This is an asynchronous operation.
    TriggerRefactor(ctx context.Context, req RefactorRequest) error
}

type RefactorRequest struct {
    UserID      string         `json:"user_id"`
    SessionID   string         `json:"session_id"`
    Policy      RefactorPolicy `json:"policy"`
}

type RefactorPolicy struct {
    SimilarityThreshold float32 `json:"similarity_threshold"` // e.g., 0.85
    MaxTopics           int     `json:"max_topics"`           // e.g., 10
    MinPRsForCluster    int     `json:"min_prs_for_cluster"`  // e.g., 3
}

// MemoryStore is the port for physical persistence (implemented by Adapters).
type MemoryStore interface {
    Save(ctx context.Context, node ContextNode) error
    BatchSave(ctx context.Context, nodes []ContextNode) error
    Get(ctx context.Context, id string) (ContextNode, error)
    GetByUserID(ctx context.Context, userID string, filters RetrievalFilters) ([]ContextNode, error)
    
    // VectorSearch performs similarity search. 
    // Adapters without native vector support must implement this via brute-force or external index.
    VectorSearch(ctx context.Context, userID string, vector []float32, filters RetrievalFilters, limit int) ([]ContextNode, error)
    
    UpdateLineage(ctx context.Context, nodeID string, parentID string, childrenIDs []string) error
}
```

---

## 4. Engine Layer (`engine.go`)

The Engine contains the cognitive logic. It depends on the `MemoryStore` and injected AI invokers.

```go
package memory

import "context"

// LLMInvoker abstracts LLM calls. The Kernel Net will implement this 
// to route calls through the Petri Net's InvokeLLM transition.
type LLMInvoker interface {
    GenerateSummary(ctx context.Context, prompt string, payloads []map[string]any) (string, error)
    ExtractTopics(ctx context.Context, payloads []map[string]any) ([]string, error)
}

// Embedder abstracts embedding generation.
type Embedder interface {
    Embed(ctx context.Context, text string) ([]float32, error)
    EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
}

// RefactoringEngine handles the transformation of chronological trees 
// into the semantic forest.
type RefactoringEngine struct {
    store    MemoryStore
    llm      LLMInvoker
    embedder Embedder
}

func NewRefactoringEngine(store MemoryStore, llm LLMInvoker, embedder Embedder) *RefactoringEngine {
    return &RefactoringEngine{store: store, llm: llm, embedder: embedder}
}

// ExecuteRefactor is the core offline logic:
// 1. Fetch all raw PRs for the session.
// 2. Embed them.
// 3. Cluster them by semantic similarity (using the policy threshold).
// 4. For each cluster, ask LLM to generate a Topic Summary.
// 5. Create new NodeTypeTopicCluster nodes and update parent/child lineage in the store.
func (e *RefactoringEngine) ExecuteRefactor(ctx context.Context, req RefactorRequest) error {
    // Implementation details for the AI team:
    // - Fetch PRs using store.GetByUserID
    // - Use e.embedder.EmbedBatch
    // - Implement a simple clustering algorithm (e.g., agglomerative or DBSCAN)
    // - Use e.llm.ExtractTopics and e.llm.GenerateSummary
    // - Use store.UpdateLineage to rewire the tree
    return nil
}
```

---

## 5. Adapter Layer (`adapters/`)

Implementations of the `MemoryStore` interface.

### 5.1 SQLite Adapter (`adapters/sqlite.go`) - *MVP Target*
```go
package adapters

// SQLiteStore implements MemoryStore using SQLite.
// Schema requirements for the AI team:
// CREATE TABLE context_nodes (
//     id TEXT PRIMARY KEY,
//     user_id TEXT NOT NULL,
//     type TEXT NOT NULL,
//     parent_id TEXT,
//     children_ids TEXT, -- JSON array
//     payload TEXT,      -- JSON blob
//     embedding BLOB,    -- Float32 array serialized
//     metadata TEXT,     -- JSON blob
//     created_at DATETIME
// );
// CREATE INDEX idx_user_type ON context_nodes(user_id, type);

type SQLiteStore struct {
    db *sql.DB
    // Add vector search extension setup if using sqlite-vss
}

// VectorSearch implementation note: 
// If sqlite-vss is not used, implement a fallback that loads all embeddings 
// for the user into memory and calculates cosine similarity in Go. 
// This is acceptable for MVP scale (<10k nodes per user).
```

### 5.2 BBolt Adapter (`adapters/bbolt.go`) - *Embedded/Fast Target*
```go
package adapters

// BBoltStore implements MemoryStore using BBolt (key-value).
// Bucket structure:
// - "nodes" bucket: key=nodeID, value=JSON(ContextNode)
// - "user_index" bucket: key=userID, value=JSON([]nodeIDs)
// - "vector_index" bucket: key=nodeID, value=Embedding bytes
```

---

## 6. Petri Net Integration (`integration.go`)

How the AxiomFlow Kernel Net wires this up.

```go
package integration

// MemorySyscallHandler translates Petri Net SyscallRequest tokens 
// into MemorySubsystem interface calls.
type MemorySyscallHandler struct {
    subsystem memory.MemorySubsystem
}

func (h *MemorySyscallHandler) HandleRequest(ctx context.Context, req SyscallRequest) (SyscallResponse, error) {
    switch req.Type {
    case "store_memory":
        // parse payload to []ContextNode
        // call h.subsystem.StoreNodes
    case "retrieve_context":
        // parse payload to RetrievalQuery
        // call h.subsystem.RetrieveContext
    case "trigger_refactor":
        // parse payload to RefactorRequest
        // call h.subsystem.TriggerRefactor (fire and forget/async)
    }
    return SyscallResponse{}, nil
}
```

---

## 7. Directory Structure

```text
axiomflow/
├── memory/                    # The Semantic Forest Subsystem
│   ├── models.go              # Core data structures
│   ├── contract.go            # Interfaces (Ports)
│   ├── engine.go              # Tree & Refactoring logic
│   ├── integration.go         # Petri Net Syscall handler
│   └── adapters/
│       ├── sqlite.go          # SQLite implementation
│       ├── bbolt.go           # BBolt implementation
│       └── mock.go            # In-memory mock for testing
├── kernel/                    # The Petri Net Runtime
│   ├── net.go
│   └── syscalls/
│       └── memory.go          # Wires MemorySyscallHandler
└── cmd/
    └── server/
        └── main.go            # Dependency injection wiring
```

---

## 8. Implementation Phases for AI Dev Team

Execute these phases sequentially. Do not move to the next phase until the current one passes tests.

### Phase 1: Core Models & Mock Adapter
1. Implement `models.go` and `contract.go`.
2. Implement `adapters/mock.go` (an in-memory map-based store).
3. Write unit tests for `MockStore` to ensure it satisfies the `MemoryStore` interface.

### Phase 2: Engine Logic (Chronological Tree)
1. Implement the basic `StoreNodes` logic in the Engine to build a simple chronological binary tree (PR1 -> S12 -> PR3).
2. Implement `GetLineage` to walk up the tree.
3. Write integration tests using the `MockStore` to verify tree construction and lineage retrieval.

### Phase 3: SQLite Adapter & Vector Search
1. Implement `adapters/sqlite.go` with the specified schema.
2. Implement basic Cosine Similarity in Go for the `VectorSearch` fallback.
3. Write tests to store nodes in SQLite, generate dummy embeddings, and retrieve them via vector search.

### Phase 4: Refactoring Engine & LLM Integration
1. Implement `RefactoringEngine.ExecuteRefactor`.
2. Create mock implementations of `LLMInvoker` and `Embedder` for testing.
3. Write a test that feeds 10 scattered PRs into the engine, mocks the LLM to return 2 topics, and verifies the store's lineage is updated to reflect the new semantic forest structure.

### Phase 5: Petri Net Wiring
1. Implement `MemorySyscallHandler`.
2. Write a test that simulates a Petri Net transition emitting a `retrieve_context` SyscallRequest, and verifies the handler correctly returns the `SyscallResponse`.

***

### 💡 Note for the AI Dev Team
*   **Concurrency:** The `MemoryStore` implementations must be thread-safe. Use `sync.RWMutex` in the Mock/BBolt adapters, and rely on SQLite's WAL mode.
*   **Context:** Always pass and respect `context.Context` for timeouts and cancellation, especially during vector search and LLM invocations.
*   **Error Handling:** Use standard Go errors. Define specific error types in `contract.go` (e.g., `ErrNodeNotFound`, `ErrInvalidQuery`).
