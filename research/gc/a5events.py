import os, subprocess, sys
sys.argv = [sys.argv[0]] + sys.argv[1:]
import importlib.util
spec = importlib.util.spec_from_file_location("m", os.path.join(os.path.dirname(os.path.abspath(__file__)), "a5measure.py"))
src = open(spec.origin).read().split("pin = sys.argv[3]")[0]
m = {}
exec(compile(src, "a5measure", "exec"), m)

def run(name, workers, pin, events):
    data = f"{m['S']}/a5e-{name}-{workers}.data"
    env = dict(os.environ, LOW_GCWORKERS=str(workers), SOURCE_DATE_EPOCH="1700000000")
    cmd = ["perf", "record", "-q", "-o", data, "-c", "100003"] + sum((["-e", e] for e in events), []) + ["--"] + (["taskset", "-c", pin] if pin else []) + m["WORKLOADS"][name]
    subprocess.run(cmd, cwd=m["W"], env=env, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    ev = m["report"](data)
    os.remove(data)
    tot = {e.split("/")[0]: sum(v for k, v in ev.get(e.split("/")[0], {}).items() if k in m["MARK"]) for e in events}
    print(f"{name} w={workers} pin={pin or '-'} " + " ".join(f"{k}={v/1e6:.1f}M" for k, v in tot.items()), flush=True)

groups = [["cycles", "cycle_activity.stalls_total", "cycle_activity.stalls_mem_any", "cycle_activity.stalls_l1d_miss", "cycle_activity.stalls_l2_miss"],
          ["cycles", "mem_load_l3_hit_retired.xsnp_hitm", "machine_clears.memory_ordering", "mem_inst_retired.lock_loads", "resource_stalls.any"]]
for name, workers, pin in [("selfbuild", 1, ""), ("selfbuild", 2, ""), ("selfbuild", 6, "0-5"), ("selfbuild", 12, "")]:
    for g in groups:
        run(name, workers, pin, g)
