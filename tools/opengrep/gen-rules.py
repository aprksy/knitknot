#!/usr/bin/env python3
"""Filter an opengrep rules pack down to the languages a repo actually contains.

Why: the p/default pack is ~1074 rules covering C, PHP, Ruby, HCL, Scala,
Solidity, Kotlin... A Go+TypeScript repo can never fire most of them. Dead rules
make the vendored file harder to review on a bump and slower to start.

Language set is DERIVED from the repo's tracked files, not hardcoded, so the same
template works for a Go service and a TS frontend.

usage: gen-rules.py <pack.json> <out.json> [repo-root]
"""
import json
import os
import subprocess
import sys
from collections import Counter

# file extension -> opengrep language name
EXT_LANG = {
    "go": "go",
    "ts": "typescript", "tsx": "typescript", "mts": "typescript", "cts": "typescript",
    "js": "javascript", "jsx": "javascript", "mjs": "javascript", "cjs": "javascript",
    "py": "python",
    "rb": "ruby",
    "php": "php",
    "java": "java",
    "kt": "kotlin", "kts": "kotlin",
    "cs": "csharp",
    "rs": "rust",
    "c": "c", "h": "c",
    "cc": "cpp", "cpp": "cpp", "hpp": "cpp", "cxx": "cpp",
    "scala": "scala",
    "swift": "swift",
    "sh": "bash", "bash": "bash",
    "yml": "yaml", "yaml": "yaml",
    "json": "json",
    "html": "html", "htm": "html",
    "xml": "xml",
    "sql": "sql",
    "tf": "hcl", "hcl": "hcl",
    "sol": "solidity",
    "ex": "elixir", "exs": "elixir",
    "clj": "clojure", "cljs": "clojure",
    "lua": "lua",
    "pl": "perl",
    "r": "r",
    "dart": "dart",
    "cshtml": "csharp",
    "dockerfile": "dockerfile",
}

# Rules with these languages are generic/heuristic and cheap; keep them always so
# cross-cutting checks (secrets, entropy, generic patterns) survive the trim.
ALWAYS = {"generic", "regex"}


def detect(root: str) -> Counter:
    try:
        out = subprocess.run(["git", "-C", root, "ls-files"],
                             capture_output=True, text=True, check=True).stdout
    except Exception:
        out = "\n".join(
            os.path.join(dp, f) for dp, _, fs in os.walk(root) for f in fs
            if ".git" not in dp and "node_modules" not in dp)

    c = Counter()
    for line in out.splitlines():
        path = line.strip().lower()
        if not path:
            continue
        base = os.path.basename(path)
        if base in ("dockerfile", "containerfile"):
            c["dockerfile"] += 1
            continue
        ext = path.rsplit(".", 1)[-1] if "." in path else ""
        lang = EXT_LANG.get(ext)
        if lang:
            c[lang] += 1
    return c


def main() -> int:
    if len(sys.argv) < 3:
        print(__doc__, file=sys.stderr)
        return 2
    src, dst = sys.argv[1], sys.argv[2]
    root = sys.argv[3] if len(sys.argv) > 3 else "."

    counts = detect(root)
    keep_langs = {l for l in counts if l} | ALWAYS
    if not keep_langs - ALWAYS:
        print("warning: no recognised source languages found; keeping the full pack",
              file=sys.stderr)
        keep_langs = None

    d = json.load(open(src))
    rules = d.get("rules") or []
    kept, dropped = [], Counter()
    for r in rules:
        langs = {str(x).lower() for x in (r.get("languages") or [])}
        if keep_langs is None or (langs & keep_langs):
            kept.append(r)
        else:
            for x in langs or {"<none>"}:
                dropped[str(x).lower()] += 1

    d["rules"] = kept
    json.dump(d, open(dst, "w"))

    print(f"detected languages: {dict(counts.most_common(8))}")
    print(f"kept {len(kept)} of {len(rules)} rules")
    if dropped:
        print("dropped (top):", dict(dropped.most_common(8)))
    print(f"wrote {dst} ({os.path.getsize(dst) // 1024} KB)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
