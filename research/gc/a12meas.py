import os, sys, time, json, re, statistics, subprocess, shlex, threading

BUDGET = float(os.environ.get("BUDGET", "270"))
START = time.monotonic()


def load1():
    return float(open("/proc/loadavg").read().split()[0])


def wait_quiet(limit=6.0):
    while load1() > limit:
        if time.monotonic() - START > BUDGET - 30:
            return False
        time.sleep(5)
    return True


def parse_trace(err):
    rows = []
    for line in err.splitlines():
        if not line.startswith("gc "):
            continue
        v = {}
        for f in line.split()[2:]:
            k, _, x = f.partition("=")
            if x.isdigit():
                v[k] = int(x)
        rows.append(v)
    return rows


def pct(s, q):
    return s[min(len(s) - 1, int(q * len(s)))]


def run(spec, cwd):
    parts = shlex.split(spec)
    env = dict(os.environ)
    while parts and "=" in parts[0] and not parts[0].startswith("/"):
        k, _, v = parts.pop(0).partition("=")
        env[k] = v
    t0 = time.monotonic()
    p = subprocess.Popen(parts, stdout=subprocess.PIPE, stderr=subprocess.PIPE, env=env, cwd=cwd)
    box = {}
    t = threading.Thread(target=lambda: box.update(out=p.stdout.read()))
    t.start()
    err = p.stderr.read().decode(errors="replace")
    t.join()
    out = box["out"].decode(errors="replace")
    _, status, ru = os.wait4(p.pid, 0)
    r = {"wall": time.monotonic() - t0, "cpu": ru.ru_utime + ru.ru_stime, "rss_mb": ru.ru_maxrss / 1024, "status": status}
    f = re.search(r"p99 (\d+) us, max (\d+) us", out)
    if f:
        r.update(fp99=int(f.group(1)) / 1000, fmax=int(f.group(2)) / 1000)
    rows = parse_trace(err)
    if rows:
        def col(k):
            return sorted(x.get(k, 0) / 1000 for x in rows)
        ps = col("pause_us")
        sw = col("sweep_us")
        mk = col("mark_us")
        r.update(n=len(ps), pmean=statistics.mean(ps), pmed=pct(ps, 0.5), p99=pct(ps, 0.99), pmax=ps[-1],
                 smean=statistics.mean(sw), s99=pct(sw, 0.99), smax=sw[-1], mmean=statistics.mean(mk), m99=pct(mk, 0.99),
                 workers=statistics.mean(x.get("workers", 0) for x in rows))
        if any("release_us" in x for x in rows):
            rl = col("release_us")
            r.update(rmean=statistics.mean(rl), rmax=rl[-1])
            sp = col("sweepw_us")
            r.update(swmean=statistics.mean(sp), swmax=sp[-1])
    return r


def main():
    name, pairs, cwd = sys.argv[1], int(sys.argv[2]), sys.argv[3]
    a, b = sys.argv[4], sys.argv[5]
    path = os.path.join(os.environ["OUT"], name + ".jsonl")
    done = 0
    if os.path.exists(path):
        done = sum(1 for _ in open(path)) // 2
    for i in range(done, pairs):
        if time.monotonic() - START > BUDGET:
            break
        if not wait_quiet():
            print("load stayed above 6; stopping at pair", i, flush=True)
            break
        order = [("base", a), ("n", b)] if i % 2 == 0 else [("n", b), ("base", a)]
        recs = []
        for side, spec in order:
            l0 = load1()
            r = run(spec, cwd)
            r["side"], r["pair"], r["load"], r["load_after"] = side, i, l0, load1()
            recs.append(r)
            print(name, side, json.dumps({k: (round(v, 3) if isinstance(v, float) else v) for k, v in r.items()}), flush=True)
        with open(path, "a") as f:
            for r in recs:
                f.write(json.dumps(r) + "\n")
    print("pairs done:", sum(1 for _ in open(path)) // 2 if os.path.exists(path) else 0)


main()
