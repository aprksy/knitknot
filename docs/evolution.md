# Phases

1. Library
2. Mini DB
3. Distributed

| Component | Phase 1: Library | Phase 2: Mini DB | Phase 3: Distributed |
|---|---|---|---|
| Query API | Go methods | + CLI + HTTP | + gRPC, Explain Plan |
| Query Engine | Naive traversal | Planner + cost model | Parallel, federated |
| Graph Core | In-memory | Delegates to file | Shard-aware proxy |
| Storage Adapter | memory.Storage | file.Storage | network.Storage |
| Index Manager | None | Inverted index (on disk) | Distributed indexing |
| Cache | None | LRU in-process | Redis / embedded cluster |
| Views | None | On-demand bipartite | Stream-updated |
| Server | N/A | main.go + HTTP | Auto-clustering |
| Extensibility | Plugins via interface | Custom scripts | Sharding, consensus |