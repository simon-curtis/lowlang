import threading, os, sys, time, json, re, statistics, subprocess

def run(argv, env_extra, cwd):
    env = dict(os.environ)
    env.update(env_extra)
    t0 = time.monotonic()
    p = subprocess.Popen(argv, stdout=subprocess.PIPE, stderr=subprocess.PIPE, env=env, cwd=cwd)
    box = {}
    t = threading.Thread(target=lambda: box.update(out=p.stdout.read()))
    t.start()
    err = p.stderr.read()
    t.join()
    out = box["out"]
    _, status, ru = os.wait4(p.pid, 0)
    wall = time.monotonic() - t0
    pauses, workers = [], []
    for line in err.decode(errors="replace").splitlines():
        m = re.search(r"pause_us=(\d+)", line)
        if m:
            pauses.append(int(m.group(1)) / 1000)
            w = re.search(r"workers=(\d+)", line)
            if w:
                workers.append(int(w.group(1)))
    r = {"wall": wall, "cpu": ru.ru_utime + ru.ru_stime, "rss_mb": ru.ru_maxrss / 1024, "status": status}
    f = re.search(r"p99 (\d+) us, max (\d+) us", out.decode(errors="replace"))
    if f:
        r.update(fp99=int(f.group(1)) / 1000, fmax=int(f.group(2)) / 1000)
    if pauses:
        s = sorted(pauses)
        r.update(n=len(s), pmean=statistics.mean(s), p99=s[min(len(s) - 1, int(0.99 * len(s)))], pmax=s[-1],
                 workers=statistics.mean(workers) if workers else 0)
    return r

def busy():
    def snap():
        v = list(map(int, open("/proc/stat").readline().split()[1:]))
        return sum(v), v[3] + v[4]
    t0, i0 = snap()
    time.sleep(1)
    t1, i1 = snap()
    return 12 * (1 - (i1 - i0) / (t1 - t0))

def gate():
    start = time.monotonic()
    while True:
        b = busy()
        if b < 3 or time.monotonic() - start > 90:
            return b

def main():
    name, pairs, cwd = sys.argv[1], int(sys.argv[2]), sys.argv[3]
    trace = os.environ.get("TRACE") == "1"
    a = sys.argv[4:sys.argv.index("--")]
    b = sys.argv[sys.argv.index("--") + 1:]
    env = {"LOW_GCTRACE": "1"} if trace else {}
    out = {"base": [], "n": []}
    for i in range(pairs):
        pre = gate()
        order = [("base", a), ("n", b)] if i % 2 == 0 else [("n", b), ("base", a)]
        for side, argv in order:
            r = run(argv, env, cwd)
            r["busy"] = pre
            r["load"] = float(open("/proc/loadavg").read().split()[0])
            out[side].append(r)
            print(name, side, json.dumps(r), flush=True)
    with open(f"{os.environ['OUT']}/{name}.json", "w") as f:
        json.dump(out, f)

main()
