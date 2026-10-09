# Evidence Round-Trip — Jupyter Producer

Jupyter is the batch processor: it filters/annotates the (large) SIEM down
to symptom-relevant events and emits a **case-entity CSV** that knitknot
accumulates into its evidence layer. knitknot never sees raw SIEM — only
this flattened exchange format.

## The exchange format

CSV with header `type,value[,context][,order]` — the same model
`correlate`/`coverage` read one-shot:

| Column | Meaning |
| --- | --- |
| `type` | Observable type (`domain`, `ip`, `sha256`, `url`, … — see the [guide](GUIDE.md#types-and-aliases)) or `technique` |
| `value` | The observable value, or an ATT&CK technique **name** (e.g. `Credential Dumping`) |
| `context` | Optional free text (sensor, note); report-only, not persisted |
| `order` | Optional logical order (integer, ascending = earlier); default is row order |

Rules that matter:

- **One row per entity.** A multi-field SIEM event becomes multiple rows
  (one per observable).
- **`order` is logical, not wall-clock.** Sequence numbers, analyst triage
  order, window order — whatever is consistent across windows. Coverage's
  ordered view consumes it; inconsistent numbering is noise.
- **`technique` rows are analyst/Jupyter annotations.** knitknot never
  infers a technique from behavior — if the notebook (or analyst) cannot
  attribute one, emit no row and the `attack-pattern` dimension stays empty.
- The CSV is validated on import: unknown types and missing values are
  row-named errors, never silently dropped.

## Producer snippet

```python
import csv

# rows: filtered SIEM hits, e.g. [{"kind": "domain", "value": "...",
#   "note": "...", "seq": 1}, {"kind": "technique", "value": "Credential Dumping", ...}]
def emit_case_csv(rows, path):
    with open(path, "w", newline="") as f:
        w = csv.writer(f)
        w.writerow(["type", "value", "context", "order"])
        for r in rows:
            w.writerow([r["kind"], r["value"].strip(), r.get("note", ""), r.get("seq", "")])

emit_case_csv([
    {"kind": "domain", "value": "tutorial-c2.example.com", "note": "proxy log", "seq": 1},
    {"kind": "technique", "value": "Credential Dumping", "note": "analyst annotation", "seq": 2},
], "window-3.csv")
```

## The round-trip

Each window's CSV is one import; evidence **accumulates** by canonical
identity (re-imports merge, first-seen `order` wins):

```bash
knitknot import --format evidence --source window-3 -f case.gob window-3.csv
# -- Evidence: 2 added, 0 merged (source window-3)

knitknot coverage --from-graph -f case.gob --target "intrusion-set:Tutorial APT"
knitknot coverage --from-graph --source window-3 -f case.gob --target "intrusion-set:Tutorial APT"
```

`--from-graph` scores the accumulated set (default: all windows);
`--source` restricts to one window's first-seen evidence. The same graph
holds the CTI reference (distinguished by `source_feed`) — co-residency is
intentional, so keep `--source` values disciplined per window.
