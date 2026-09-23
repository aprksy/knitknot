# CTI demo

Five beats on the `demo-campaign` fixture: **ingest** a STIX 2.1 bundle as
`northwind-cert`, **investigate** it with three DSL queries (campaign→malware
`uses`, indicator→malware `indicates`, `Where` on `pattern_type`), **evolve**
it by re-importing an updated bundle as `first-response-ir` (confidence set,
one indicator revoked), **history** on the bumped indicator showing both
sourced snapshots, and **share** via STIX + DOT/SVG export.

Prereqs: Go and python3; Graphviz (`dot`) optional — the script notes and
skips SVG when absent.

Run from the repo root: `bash demo/cti-demo.sh`

Artifacts (repo root unless noted): `/tmp/knitknot-demo` binary,
`demo.gob` graph, `demo-stix.json` STIX bundle, `demo.dot`, `demo.svg`
(if Graphviz); the temp evolved bundle removes itself. `rm -f
/tmp/knitknot-demo demo.gob demo-stix.json demo.dot demo.svg` to clean up.
