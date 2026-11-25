import json, sys, statistics, random

random.seed(1)
med = statistics.median

def load(names):
    out = {"base": [], "n": []}
    for nm in names:
        d = json.load(open(nm))
        for k in out:
            out[k] += d[k]
    return out

def boot(a, b, reps=4000):
    ds = []
    for _ in range(reps):
        ds.append(med(random.choices(b, k=len(b))) - med(random.choices(a, k=len(a))))
    ds.sort()
    return ds[int(0.025 * reps)], ds[int(0.975 * reps)]

def aa(a):
    return abs(med(a[0::2]) - med(a[1::2]))

def row(metric, d):
    a = [r[metric] for r in d["base"] if metric in r]
    b = [r[metric] for r in d["n"] if metric in r]
    if not a or not b:
        return
    ma, mb = med(a), med(b)
    lo, hi = boot(a, b)
    noise = max(aa(a), aa(b))
    from math import comb
    pairs = list(zip(a, b))
    untied = [(x, y) for x, y in pairs if x != y]
    lower = sum(1 for x, y in untied if y < x)
    k = min(lower, len(untied) - lower)
    p = min(1.0, 2 * sum(comb(len(untied), i) for i in range(k + 1)) / 2 ** len(untied)) if untied else 1.0
    ratio = med([y / x for x, y in pairs if x])
    sig = "yes" if p < 0.05 and abs(mb - ma) > noise else "no"
    print(f"| {metric} | {ma:.3g} [{min(a):.3g}..{max(a):.3g}] | {mb:.3g} [{min(b):.3g}..{max(b):.3g}] | {100*(mb-ma)/ma:+.1f}% | {lo:+.3g}..{hi:+.3g} | {noise:.3g} | {lower}/{len(untied)} p={p:.2g} | {ratio:.3f} | {sig} |")

d = load(sys.argv[2:])
print(f"### {sys.argv[1]} (n={len(d['base'])} pairs)")
print("| metric | base median [min..max] | branch median [min..max] | diff | 95% CI of diff | A/A noise | branch lower in pairs (sign test) | median paired ratio | beyond noise |")
print("|---|---|---|---|---|---|---|---|---|")
for m in ["wall", "cpu", "rss_mb", "pmean", "p99", "pmax", "n", "workers", "fp99", "fmax", "busy", "load"]:
    row(m, d)
