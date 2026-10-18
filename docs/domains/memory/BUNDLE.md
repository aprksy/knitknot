# memory-bundle format (v1)

The interchange format for loading memory graphs into knitknot. Both sides
(producer and importer) implement exactly this shape; anything else is
rejected.

```json
{
  "type": "memory-bundle",
  "version": 1,
  "nodes": [
    {
      "memory_id": "u-alice",
      "label": "user_profile",
      "props": { "user_id": "alice" }
    },
    {
      "memory_id": "turn-1",
      "label": "pr_turn",
      "props": {
        "user_id": "alice",
        "payload": { "prompt": "...", "response": "..." },
        "created_at": 1760572800,
        "format": "v1"
      }
    },
    {
      "memory_id": "t-python",
      "label": "tag",
      "props": { "name": "python" }
    }
  ],
  "edges": [
    { "from": "turn-1", "to": "u-alice", "kind": "child-of", "props": {} },
    { "from": "turn-1", "to": "t-python", "kind": "tagged", "props": {} }
  ]
}
```

## Nodes

- `memory_id` (required, string): stable cross-system key. Upsert identity.
- `label` (required, string): one of `user_profile`, `session_summary`,
  `pr_turn`, `topic_cluster`, `tag`. Unknown labels are rejected.
- `props` (required, object): `user_id` required on all content nodes
  (everything except `tag`); time props as epoch numbers or RFC3339 strings
  (both filter chronologically); `format` `"v1"` (default) or `"v2"` per the
  payload-split rules.

## Edges

- `from`, `to` (required, strings): `memory_id` references, resolved to
  storage IDs on import.
- `kind` (required, string): `child-of`, `similar-to`, `supersedes`,
  `tagged`. Unknown kinds are rejected.
- `props` (optional, object): kind-required props must be present —
  `similar-to` needs `score`, `model`, `observed_at`.

## Import semantics

- **Upsert by `memory_id`**: existing node → merge props, append a snapshot;
  new `memory_id` → create. History is never rewritten.
- **Edges resolve after all nodes stage**; a dangling `memory_id` aborts the
  whole import with zero partial writes (fail-before-mutate, mirroring the
  STIX importer).
- **Strict vocabulary**: unknown labels or kinds are errors naming the
  value, never silent drops — a bundle that parses but means something else
  is worse than a rejection.
- `similar-to` edges are normally written back by the indexer, but the
  importer accepts them like any edge (same validation).
