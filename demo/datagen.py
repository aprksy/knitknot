#!/usr/bin/env python3
"""Deterministic synthetic-dataset generator emitting knitknot write DSL.

Self-contained (stdlib only), reproducible: same seed = same dataset.
The knotted part — which rows connect to which — is decided here;
knitknot owns the graph semantics (GOVERNANCE rule: no LLM, no stats).

IDs: knitknot mints sequential ids (n1, n2, ...) per fresh session, so
the caller must feed this stream into ONE fresh empty session (the
stretch driver does). The k-th ADDNODE line becomes node n<k>.

Usage: datagen.py --nodes 3000 [--seed 7] ... > stream.txt
"""

import argparse
import random


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--nodes", type=int, default=3000, help="number of Person nodes")
    ap.add_argument("--companies", type=int, default=30)
    ap.add_argument("--skills", type=int, default=25)
    ap.add_argument("--seed", type=int, default=7)
    ap.add_argument("--knows-avg", type=float, default=1.2, help="avg knows edges per person")
    args = ap.parse_args()
    rng = random.Random(args.seed)

    c, s, n = args.companies, args.skills, args.nodes
    pid = lambda i: f"n{c + s + i + 1}"  # noqa: E731  (persons come after corp+skill adds)

    lines = []
    # Companies and skills first — their ids fall out of the creation order.
    for i in range(c):
        rev = rng.randint(-50, 500)
        lines.append(f"ADDNODE Company name=corp{i:03d} employees={rng.randint(10, 500)} revenue_km={rev}")
    for i in range(s):
        lines.append(f"ADDNODE Skill name=skill{i:03d}")

    ages = sorted(int(max(18, min(75, rng.gauss(38, 11)))) for _ in range(n))
    for i in range(n):
        lines.append(f"ADDNODE Person name=person{i:05d} age={ages[i]} city=city{i % 40:02d}")

    # Relationship layer, deduped (edge ids are from->to@kind).
    has_skill, pair_seen = [], set()
    for p in range(n):
        for _ in range(rng.randint(0, 3)):
            so = rng.randint(0, s - 1)
            if (p, so) in pair_seen:
                continue
            pair_seen.add((p, so))
            has_skill.append((p, so))
    knows, knows_seen = [], set()
    for p in range(n):
        deg = max(0, int(round(rng.gauss(args.knows_avg, 0.7))))
        for _ in range(deg):
            q = (p + rng.randint(1, max(2, n // 2))) % n
            if p == q or (p, q) in knows_seen:
                continue
            knows_seen.add((p, q))
            knows.append((p, q))

    import sys
    print(f"datagen seed={args.seed}: {n} persons, {c} companies, {s} skills,"
          f" {len(has_skill)} has_skill, {len(knows)} knows", file=sys.stderr)  # stdout must stay pure DSL
    lines.extend(
        [f"CONNECT {pid(p)} --knows strength={rng.randint(1, 9)}--> {pid(q)}" for p, q in knows])
    lines.extend(
        [f"CONNECT {pid(p)} --has_skill--> n{c + so + 1}" for p, so in has_skill])
    # UPDATEs: sprinkle verified tenure across people.
    for p in range(0, n, 17):
        lines.append(f"UPDATE NODE {pid(p)} verified=true since=20{rng.randint(10, 24)}")
    for line in lines:
        print(line)


if __name__ == "__main__":
    main()
