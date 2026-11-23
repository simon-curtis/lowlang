import json, os, resource, statistics, subprocess, sys, time, argparse
ap = argparse.ArgumentParser()
ap.add_argument("--runs", type=int, default=10)
ap.add_argument("--cwd", default=".")
ap.add_argument("--out", default=None)
ap.add_argument("--tag", default="")
ap.add_argument("spec", nargs="+", help="name=ENV1=v,ENV2=v::cmd args (use ; to split)")
a = ap.parse_args()

def parse(spec):
    name, rest = spec.split("=", 1)
    envs, cmd = rest.split("::", 1)
    env = {}
    for kv in envs.split(","):
        if kv:
            k, v = kv.split(":", 1); env[k] = v
    return name, env, cmd.split()

def one(env, cmd):
    e = dict(os.environ); e.update(env); e["LOW_GCTRACE"] = "1"
    before = resource.getrusage(resource.RUSAGE_CHILDREN)
    t = time.perf_counter()
    p = subprocess.Popen([os.path.join(os.environ["S"], "maxrss")] + cmd, env=e, cwd=a.cwd, stdout=subprocess.DEVNULL, stderr=subprocess.PIPE, text=True)
    err = p.communicate()[0] if False else p.stderr.read()
    _, status, ru = os.wait4(p.pid, 0)
    w = time.perf_counter() - t
    gcs = []
    for l in err.splitlines():
        if l.startswith("gc "):
            d = {}
            for f in l.split()[2:]:
                k, v = f.split("="); d[k] = int(v)
            gcs.append(d)
    s = lambda k: sum(g.get(k, 0) for g in gcs)
    mr = [int(l.split("=")[1]) for l in err.splitlines() if l.startswith("MAXRSS_KB=")]
    return dict(rc=status, wall=w, cpu=ru.ru_utime + ru.ru_stime, rss_mb=mr[0] / 1024, marked_mb=s("live_kb")/1024, gcs=len(gcs),
                pause_ms=s("pause_us")/1e3, mark_ms=s("mark_us")/1e3, hash_ms=s("hash_us")/1e3, minors=s("minor"),
                mark_per_gc_us=s("mark_us")/max(1,len(gcs)), sweep_ms=s("sweep_us")/1e3,
                stop_ms=s("stop_us")/1e3, restart_ms=s("restart_us")/1e3,
                gc_cpu_ms=sum(g.get("pause_us",0)*max(1,g.get("workers",1)) for g in gcs)/1e3,
                maxpause_ms=max([g.get("pause_us",0) for g in gcs] or [0])/1e3,
                maxheap_mb=max([g.get("heap_kb",0) for g in gcs] or [0])/1024)

specs = [parse(x) for x in a.spec]
res = {n: [] for n, _, _ in specs}
for r in range(a.runs):
    order = specs if r % 2 == 0 else specs[::-1]
    for n, env, cmd in order:
        res[n].append(one(env, cmd))
keys = ["wall","cpu","rss_mb","gcs","minors","mark_ms","hash_ms","mark_per_gc_us","marked_mb","maxpause_ms"]
def q(v):
    v = sorted(v); n = len(v)
    return statistics.median(v), v[n//4], v[(3*n)//4]
out = {}
for n in res:
    rows = res[n]
    bad = [x["rc"] for x in rows if x["rc"] != 0]
    summ = {k: q([x[k] for x in rows]) for k in keys}
    out[n] = summ
    print(f"{a.tag}{n:22s}" + ("BAD_RC " if bad else "") + " ".join(f"{k}={summ[k][0]:.3g}[{summ[k][1]:.3g},{summ[k][2]:.3g}]" for k in keys))
if a.out:
    with open(a.out, "a") as f:
        f.write(json.dumps(dict(tag=a.tag, runs=a.runs, raw=res)) + "\n")
