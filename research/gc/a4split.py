import re, statistics as st, sys
T = "a4-out/traces"
NAMES = ["gcpause", "shuffle", "selfbuild", "lint-pkg", "playground-wasm", "selfbuild-wdef"]

def gcs(path):
    return [{k: int(v) for k, v in re.findall(r"(\w+)=(\d+)", l)} for l in open(path) if l.startswith("gc ")]

def a2d_hash_ns_per_word(name):
    try:
        lines = open(f"a2d-out/{name}.txt").read().split("\n")
    except FileNotFoundError:
        return None
    head = lines[0].split()
    rows = [dict(zip(head, map(float, l.split()))) for l in lines[1:] if l.strip()]
    v = [r["hash_ns"] / r["live_w"] for r in rows if r["live_w"] > 0]
    return st.median(v) if v else None

print("workload off_gcs off_mark full_n full_mark full_rehash minor_n minor_hash minor_resid  hash_ns/sticky_w a2d_hash_ns/w  minor_resid/gc off_mark/gc  dead_frac_at_full")
for n in NAMES:
    off, mi = gcs(f"{T}/{n}.off.trace"), gcs(f"{T}/{n}.minor.trace")
    full = [g for g in mi if not g.get("minor")]
    minor = [g for g in mi if g.get("minor")]
    fm = sum(g["mark_us"] - g["hash_us"] for g in full) / 1e3
    fr = sum(g["hash_us"] for g in full) / 1e3
    mh = sum(g["hash_us"] for g in minor) / 1e3
    mr = sum(g["mark_us"] - g["hash_us"] for g in minor) / 1e3
    per = []
    for prev, g in zip(mi, mi[1:]):
        if g.get("minor") and prev["live_kb"] > 1024:
            per.append(g["hash_us"] * 1e3 / (prev["live_kb"] * 128))
    dead = []
    for prev, g in zip(mi, mi[1:]):
        if not g.get("minor") and prev.get("minor") and prev["live_kb"] > 0:
            dead.append(1 - g["live_kb"] / prev["live_kb"])
    a2 = a2d_hash_ns_per_word(n.replace("-wdef", ""))
    print(f"{n} {len(off)} {sum(g['mark_us'] for g in off)/1e3:.0f} {len(full)} {fm:.0f} {fr:.0f} {len(minor)} {mh:.0f} {mr:.0f}  "
          f"{st.median(per) if per else float('nan'):.2f} {a2 if a2 is not None else float('nan'):.2f}  "
          f"{mr*1e3/max(1,len(minor)):.0f} {sum(g['mark_us'] for g in off)/max(1,len(off)):.0f}  "
          f"{st.median(dead) if dead else float('nan'):.2f}")
