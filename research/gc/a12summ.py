import json, sys, statistics
from math import comb

med = statistics.median
METRICS = ["wall", "cpu", "rss_mb", "pmean", "pmed", "p99", "pmax", "mmean", "m99", "smean", "s99", "smax",
           "swmean", "swmax", "rmean", "rmax", "n", "workers", "fp99", "fmax", "load", "load_after"]


def load(paths):
    out = {"base": [], "n": []}
    for p in paths:
        for line in open(p):
            r = json.loads(line)
            out[r["side"]].append(r)
    for k in out:
        out[k].sort(key=lambda r: r["pair"])
    return out


def row(metric, d):
    pairs = [(x[metric], y[metric]) for x, y in zip(d["base"], d["n"]) if metric in x and metric in y]
    if not pairs:
        return
    a = [x for x, _ in pairs]
    b = [y for _, y in pairs]
    ma, mb = med(a), med(b)
    untied = [(x, y) for x, y in pairs if x != y]
    lower = sum(1 for x, y in untied if y < x)
    k = min(lower, len(untied) - lower)
    p = min(1.0, 2 * sum(comb(len(untied), i) for i in range(k + 1)) / 2 ** len(untied)) if untied else 1.0
    spread_a = max(a) - min(a)
    ratio = med([y / x for x, y in pairs if x]) if all(x for x, _ in pairs) else float("nan")
    print(f"| {metric} | {ma:.4g} [{min(a):.4g}..{max(a):.4g}] | {mb:.4g} [{min(b):.4g}..{max(b):.4g}] | {ratio:.3f} | {lower}/{len(untied)} p={p:.2g} |")


d = load(sys.argv[2:])
bad = [r for k in d for r in d[k] if r.get("status")]
print(f"### {sys.argv[1]} ({len(d['base'])} pairs; nonzero exit: {len(bad)})")
print("| metric | base median [min..max] | branch median [min..max] | median paired ratio | branch lower in pairs |")
print("|---|---|---|---|---|")
for m in METRICS:
    row(m, d)
