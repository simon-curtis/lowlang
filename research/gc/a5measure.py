import os, re, subprocess, sys

S = os.environ["S"]
W = os.environ["W"]
OUT = os.environ.get("A5_OUT", os.path.join(os.path.dirname(os.path.abspath(__file__)), "a5-out"))
MARK = {"rt_mark", "rt_mark_loop", "rt_mark_solo", "rt_scan_range", "rt_find", "rt_scan_g", "rt_scan_region",
        "rt_gcw_roots", "rt_mark_grow", "rt_mark_commit"}
DIST = {"rt_gcw_acquire", "rt_gcw_donate", "rt_gcw_publish", "rt_gcw_take_locked", "rt_gcw_lock_acquire",
        "rt_gcw_sleep", "rt_gcw_join", "rt_gcw_join_busy", "rt_gcw_signal", "rt_gc_wait", "rt_gcw_enter",
        "rt_gcw_finish", "rt_gcw_resume", "rt_gcw_pool_grow", "rt_gcw_start_mark", "rt_gcw_end_mark"}
WORKLOADS = {
    "gcpause": [f"{S}/bin/gcpause"],
    "selfbuild": [f"{S}/g3", "build", "compiler/low", "-o", f"{S}/tmp-out"],
    "shuffle": [f"{S}/bin/shuffle"],
}


def report(data):
    out = subprocess.run(["perf", "report", "-i", data, "--stdio", "--sort", "sym", "-n"],
                         capture_output=True, text=True).stdout
    events, cur, total = {}, None, 0
    for line in out.splitlines():
        m = re.match(r"# Samples: .* of event '([^']+)'", line)
        if m:
            cur = events.setdefault(m.group(1).split("/")[0], {})
            continue
        m = re.match(r"# Event count \(approx.\): (\d+)", line)
        if m:
            total = int(m.group(1))
            continue
        m = re.match(r"\s*([\d.]+)%\s+\d+\s+\[[.k]\]\s+(\S+)", line)
        if m and cur is not None:
            cur[m.group(2)] = cur.get(m.group(2), 0) + float(m.group(1)) / 100 * total
    return events


def run(name, workers, pin):
    data = f"{S}/a5-{name}-{workers}.data"
    env = dict(os.environ, LOW_GCWORKERS=str(workers), LOW_GCTRACE="1", SOURCE_DATE_EPOCH="1700000000")
    p = subprocess.run(["perf", "record", "-q", "-o", data,
                        "-e", "cpu-clock", "-e", "longest_lat_cache.miss", "-e", "cycles", "-e", "cycle_activity.stalls_l3_miss", "-c", "100003",
                        "--"] + (["taskset", "-c", pin] if pin else []) + WORKLOADS[name], cwd=W, env=env, stdout=subprocess.DEVNULL, stderr=subprocess.PIPE, text=True)
    gcs = [dict((k, int(v)) for k, v in re.findall(r"(\w+)=(\d+)", l)) for l in p.stderr.splitlines() if l.startswith("gc ")]
    open(f"{OUT}/{name}-{workers}.trace", "w").write(p.stderr)
    mark_us = sum(g["mark_us"] for g in gcs)
    used = [g.get("workers", 1) for g in gcs]
    wmean = sum(g["mark_us"] * g.get("workers", 1) for g in gcs) / max(1, mark_us)
    ev = report(data)
    clk, miss = ev.get("cpu-clock", {}), ev.get("longest_lat_cache.miss", {})
    cyc, stall = ev.get("cycles", {}), ev.get("cycle_activity.stalls_l3_miss", {})
    mark_cyc = sum(v for k, v in cyc.items() if k in MARK)
    mark_stall = sum(v for k, v in stall.items() if k in MARK)
    os.remove(data)
    mark_cpu_us = sum(v for k, v in clk.items() if k in MARK) / 1e3
    dist_cpu_us = sum(v for k, v in clk.items() if k in DIST) / 1e3
    mark_lines = sum(v for k, v in miss.items() if k in MARK)
    rate = mark_lines / max(1, mark_us)
    util = (mark_cpu_us + dist_cpu_us) / max(1, wmean * mark_us)
    print(f"{name} w={workers} pin={pin or '-'} mark_Gcycles={mark_cyc/1e9:.2f} mark_stall_l3_Gcycles={mark_stall/1e9:.2f} stall_share={mark_stall/max(1,mark_cyc):.2f}")
    print(f"{name} w={workers} gcs={len(gcs)} mean_workers={wmean:.1f} mark_ms={mark_us/1e3:.0f} "
          f"mark_cpu_ms={mark_cpu_us/1e3:.0f} dist_cpu_ms={dist_cpu_us/1e3:.0f} dist_share={dist_cpu_us/max(1,mark_cpu_us+dist_cpu_us):.3f} "
          f"busy_util={util:.2f} mark_Mlines={mark_lines/1e6:.0f} Mlines_per_s={rate:.0f} "
          f"misses_per_us_cpu={mark_lines/max(1,mark_cpu_us):.1f}", flush=True)
    top = sorted(((v, k) for k, v in clk.items() if k.startswith("rt_g") or k.startswith("rt_m") or k.startswith("rt_s") or k.startswith("rt_f")), reverse=True)[:8]
    print("   top cpu:", " ".join(f"{k}={v/1e6:.0f}ms" for v, k in top), flush=True)


pin = sys.argv[3] if len(sys.argv) > 3 else ""
for name in sys.argv[1].split(","):
    for w in map(int, sys.argv[2].split(",")):
        run(name, w, pin)
