import ctypes, json, os, statistics, subprocess, sys, threading, time
from ctypes import wintypes

S = os.environ["S"]
W = os.environ["W"]
PERFORMANCE_CPUS = 0xFFFF

class Counters(ctypes.Structure):
    _fields_ = [("cb", wintypes.DWORD), ("faults", wintypes.DWORD),
                ("peak_ws", ctypes.c_size_t), ("ws", ctypes.c_size_t),
                ("qpp", ctypes.c_size_t), ("qp", ctypes.c_size_t),
                ("qpnp", ctypes.c_size_t), ("qnp", ctypes.c_size_t),
                ("pagefile", ctypes.c_size_t), ("peak_pagefile", ctypes.c_size_t)]

kernel = ctypes.WinDLL("kernel32", use_last_error=True)
psapi = ctypes.WinDLL("psapi", use_last_error=True)
psapi.GetProcessMemoryInfo.argtypes = [wintypes.HANDLE, ctypes.POINTER(Counters), wintypes.DWORD]
kernel.GetProcessTimes.argtypes = [wintypes.HANDLE] + [ctypes.POINTER(wintypes.FILETIME)] * 4
kernel.SetProcessAffinityMask.argtypes = [wintypes.HANDLE, ctypes.c_size_t]
kernel.GetCurrentProcess.restype = wintypes.HANDLE

def memory(handle):
    c = Counters(); c.cb = ctypes.sizeof(c)
    psapi.GetProcessMemoryInfo(handle, ctypes.byref(c), c.cb)
    return c

def cpu_seconds(handle):
    t = [wintypes.FILETIME() for _ in range(4)]
    kernel.GetProcessTimes(handle, *[ctypes.byref(x) for x in t])
    ticks = lambda f: (f.dwHighDateTime << 32) | f.dwLowDateTime
    return (ticks(t[2]) + ticks(t[3])) / 1e7

def command(workload, arm):
    if workload == "selfbuild":
        return [f"{S}/bin/q2.exe", "build", "compiler/low", "-o", f"{S}/bin/out-{arm}.exe"], W
    if workload == "lint":
        return [f"{S}/bin/q2.exe", "lint", "compiler/low"], W
    return [f"{S}/bin/{workload}.exe"], S

def trace(path):
    rows = []
    for line in open(path, errors="replace"):
        if line.startswith("gc "):
            rows.append({k: int(v) for k, v in (f.split("=") for f in line.split()[2:])})
    return rows

def run(workload, arm, settings, index):
    argv, cwd = command(workload, arm)
    env = dict(os.environ, LOW_GCTRACE="1", **settings)
    err = f"{S}/tr/{workload}.{arm}.{index}.err"
    samples = []
    with open(err, "wb") as stderr:
        start = time.perf_counter()
        child = subprocess.Popen(argv, cwd=cwd, env=env, stdout=subprocess.DEVNULL, stderr=stderr)
        handle = wintypes.HANDLE(int(child._handle))
        def sample():
            while child.poll() is None:
                samples.append(memory(handle).ws)
                time.sleep(0.005)
        sampler = threading.Thread(target=sample); sampler.start()
        code = child.wait()
        wall = time.perf_counter() - start
        sampler.join()
        final = memory(handle)
        cpu = cpu_seconds(handle)
    rows = trace(err)
    small = [r["mark_us"] for r in rows if r["live_kb"] < 4096]
    tail = samples[-21:-1] or samples
    return {
        "workload": workload, "arm": arm, "exit": code, "wall": wall, "cpu": cpu,
        "peak_ws_mb": final.peak_ws / 2**20, "peak_commit_mb": final.peak_pagefile / 2**20,
        "mean_ws_mb": statistics.fmean(samples) / 2**20 if samples else 0,
        "end_ws_mb": statistics.median(tail) / 2**20 if tail else 0,
        "gcs": len(rows), "mark_ms": sum(r["mark_us"] for r in rows) / 1e3,
        "sweep_ms": sum(r["sweep_us"] for r in rows) / 1e3,
        "pause_ms": sum(r["pause_us"] for r in rows) / 1e3,
        "max_pause_ms": max((r["pause_us"] for r in rows), default=0) / 1e3,
        "marked_mb": sum(r["live_kb"] for r in rows) / 1024,
        "small_mark_us": statistics.fmean(small) if small else 0,
        "max_heap_mb": max((r["heap_kb"] for r in rows), default=0) / 1024,
    }

def main():
    out, runs, workloads, arms = sys.argv[1], int(sys.argv[2]), sys.argv[3].split(","), {}
    for spec in sys.argv[4:]:
        name, _, pairs = spec.partition(":")
        arms[name] = dict(p.split("=") for p in pairs.split(",") if p)
    kernel.SetProcessAffinityMask(kernel.GetCurrentProcess(), PERFORMANCE_CPUS)
    os.makedirs(f"{S}/tr", exist_ok=True)
    results = []
    with open(out, "a") as log:
        for workload in workloads:
            for arm, settings in arms.items():
                run(workload, arm, settings, "warm")
            for index in range(runs):
                order = list(arms.items())
                if index % 2:
                    order.reverse()
                for arm, settings in order:
                    row = run(workload, arm, settings, index)
                    results.append(row)
                    log.write(json.dumps(row) + "\n"); log.flush()
    keys = ["wall", "cpu", "peak_ws_mb", "mean_ws_mb", "end_ws_mb", "peak_commit_mb", "gcs",
            "mark_ms", "sweep_ms", "pause_ms", "max_pause_ms", "small_mark_us", "max_heap_mb"]
    print("workload arm exit " + " ".join(keys))
    for workload in workloads:
        for arm in arms:
            mine = [r for r in results if r["workload"] == workload and r["arm"] == arm]
            exits = sorted({r["exit"] for r in mine})
            print(workload, arm, exits, " ".join(f"{statistics.median(r[k] for r in mine):.4g}" for k in keys))

main()
