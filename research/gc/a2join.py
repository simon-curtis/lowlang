import os
import re
import statistics as st
import sys

OUT = sys.argv[1] if len(sys.argv) > 1 else os.path.join(os.path.dirname(os.path.abspath(__file__)), "a2-out")
WORKLOADS = ["gcpause", "binarytrees", "selfbuild", "shuffle", "jsonpar", "strings", "maps", "doc", "wasm", "lint", "sort"]


def trace(name):
    rows = {}
    for line in open(f"{OUT}/{name}.trace"):
        if not line.startswith("gc "):
            continue
        f = dict(re.findall(r"(\w+)=(\d+)", line))
        rows[int(line.split()[1])] = {k: int(v) for k, v in f.items()}
    return rows


def pairs(name):
    lines = open(f"{OUT}/{name}.txt").read().split("\n")
    head = lines[0].split()
    for line in lines[1:]:
        if line.strip():
            yield dict(zip(head, map(float, line.split())))


def med(xs):
    return st.median(xs) if xs else float("nan")


print("workload pairs live_mb c_mark_vs_rt hash_vs_rtmark scan_vs_hash scan2_vs_hash dirty_h dirty_h2 dirty_s dirty_s2 dirty_raw post unsound proj_rt proj_rt_min proj_rt_max proj_c")
for name in WORKLOADS:
    try:
        t = trace(name)
        list(pairs(name))
    except FileNotFoundError:
        continue
    rows = []
    for p in pairs(name):
        g = t.get(int(p["gc"]) + 1)
        if not g or g["live_kb"] < 2048 or p["live_w"] == 0:
            continue
        if abs(p["live_w"] / 128 - g["live_kb"]) > 0.02 * g["live_kb"] + 64:
            print("misaligned", name, p["gc"], p["live_w"] / 128, g["live_kb"], file=sys.stderr)
            continue
        rt_ns = g["mark_us"] * 1000
        if rt_ns <= 0:
            continue
        per_word = rt_ns / p["live_w"]
        proj = (p["hash_ns"] + per_word * (p["dirty_w"] + p["young_w"])) / rt_ns
        c_ns = p["mark_ns_kb"] * p["live_w"] / 128
        rows.append(dict(p, rt_ns=rt_ns, proj=proj, cvr=c_ns / rt_ns, hvr=p["hash_ns"] / rt_ns))
    if not rows:
        print(name, 0)
        continue
    m = lambda k: med([r[k] for r in rows])
    print(f"{name} {len(rows)} {m('live_kb')/1024:.1f} {m('cvr'):.2f} {m('hvr'):.3f} {m('scan_vs_hash'):.2f} "
          f"{m('scan2_vs_hash'):.2f} {m('dirty_h'):.4f} {m('dirty_h2'):.4f} {m('dirty_s'):.4f} {m('dirty_s2'):.4f} {m('dirty_raw'):.4f} {min(r['raw_post'] for r in rows):.0f} "
          f"{int(sum(r['unsound'] for r in rows))} {m('proj'):.3f} {min(r['proj'] for r in rows):.3f} {max(r['proj'] for r in rows):.3f} {m('proj_minor_vs_full'):.3f}")
