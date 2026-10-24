# knitknot server reference (`knitknot serve`)

Serve the DSL over HTTP for out-of-process drivers (knitgen, axiomflow
over a process boundary). Requires knitknot **v0.2.0+** (the `serve`
command does not exist in v0.1.0).

Decisions behind this surface: ADR 0010. Grammar: `docs/dsl-syntax.md`.
Strategy and boundary: `docs/strategy-datagen.md` (§2 is binding —
no LLM, embeddings, or statistics in this repo, server included).

## Start

```bash
knitknot serve                                        # in-memory, loopback :8080
knitknot serve --addr 127.0.0.1:9000                  # custom loopback port
knitknot serve --store bolt:graph.db                  # durable store
knitknot serve -f graph.gob                           # gob file store
```

One engine lives for the server's lifetime. Do not run two writers
against the same bolt file. The server prints its address on start;
a non-loopback `--addr` prints a warning (see Security).

## Endpoint

`POST /v1/execute` with JSON `{"dsl": "<one statement>"}` — a read
query or a write, same grammar as the REPL.

```bash
curl -X POST localhost:8080/v1/execute -H 'Content-Type: application/json' \
  -d '{"dsl": "ADDNODE Person name='"'"'Ada'"'"' age=36"}'
# {"ok":true,"results":[],"node_id":"n1"}

curl -X POST localhost:8080/v1/execute -H 'Content-Type: application/json' \
  -d '{"dsl": "Find('"'"'Person'"'"').Where('"'"'n.name'"'"','"'"'='"'"','"'"'Ada'"'"')"}'
# {"ok":true,"results":[{"n":{"id":"n1","label":"Person",
#   "props":{"age":36,"name":"Ada"}}}]}
```

### Response fields

| Field | Present when | Meaning |
|---|---|---|
| `ok` | always | `false` means nothing was executed |
| `results` | always (possibly `[]`) | rows: `{var: {id, label, props}}` |
| `node_id` | node writes | `ADDNODE` → new id; `UPDATE`/`DELETE NODE` → target id |
| `edge_id` | edge writes | `CONNECT`/`UPDATE`/`DELETE EDGE`, storage form `from->to@kind` |
| `verb` | `DEFINE` | the registered verb name |
| `error` | `ok:false` | human-readable reason (parse, missing node/edge, bad body) |

### Statuses

| Status | When |
|---|---|
| 200 | executed (reads and writes) |
| 400 | bad body (not JSON / no `dsl`), unparsable DSL, or execution error — details in `error`, nothing mutated |
| 405 | anything that is not POST |

DSL value rules apply over the wire exactly as in the REPL:
`'quoted strings'` (quote multi-word values — `name='John Doe'`),
signed numbers (`age=36`, `score=-5`), bare words kept as strings
(`active=true` stays `"true"`).

## Storage

| Flag | Backend | Durability |
|---|---|---|
| (none) | in-memory | lost on stop |
| `-f file.gob` | gob file | loaded at start; save on exit paths as usual |
| `--store bolt:file.db` | BoltDB | crash-safe, survives restarts |

The server reuses the CLI's store resolution (`resolveStoreBackend`),
so any backend the CLI accepts works here.

## IDs

Nodes are minted `n1, n2, …` in creation order **per fresh store**;
a reopened durable store resumes past the highest id seen. Edge ids
are deterministic: `from->to@kind`.

Generators: prefer reading `node_id` from each write response and
feeding it into the next statement — that is robust everywhere. Only
predict ids (`n(k+1)` for the k-th `ADDNODE`) when you own the whole
session against a fresh store and know the exact creation order
(`demo/datagen.py` works this way). Never assume `n1` on a reused
store.

## Security

There is **no auth and no TLS**. The default `--addr 127.0.0.1:8080`
binds loopback only — keep it that way. Binding anything else prints
a warning and means anyone who can reach the port can rewrite your
graph. Remote or multi-user use requires authentication first, which
is not implemented (explicitly deferred, not forgotten).

## Limits (v0.2.0)

- One statement per request; no multi-statement transactions.
- Exporters exist (`csv`, `postgres`, `parquet`, `dot`) but none are
  served over HTTP yet.
- No server-side batching: bulk loads are N requests; in-process
  callers should use `GraphEngine.Execute` directly. Measured rates
  (same machine, default in-memory store):
  - in-process `ParseProgram`+`Execute`: ~100–160K writes/s
    (`pkg/graph/execute_stress_test.go`: 10,150 nodes in ~89ms,
    20,000 edges in ~174ms).
  - REPL loopback (piped stdin, mixed add/connect/update):
    ~4K statements/s (`demo/datagen-stretch.sh`: 11,156 in ~2.6s;
    29,721 in ~7.4s).
  - Loopback HTTP has no measured figure — do not quote the REPL
    number for it (it pays a round-trip and JSON per statement).
