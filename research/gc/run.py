import os, resource, subprocess, sys, time
env = dict(os.environ)
args = sys.argv[1:]
t = time.perf_counter()
p = subprocess.run(args, env=env, stdout=subprocess.DEVNULL, stderr=subprocess.PIPE, text=True)
w = time.perf_counter() - t
r = resource.getrusage(resource.RUSAGE_CHILDREN)
gcs = [l for l in p.stderr.splitlines() if l.startswith("gc ")]
def tot(k):
    s = 0
    for l in gcs:
        for f in l.split():
            if f.startswith(k + "="): s += int(f.split("=")[1])
    return s
mx = lambda k: max([int(f.split("=")[1]) for l in gcs for f in l.split() if f.startswith(k+"=")] or [0])
print(f"rc={p.returncode} wall={w:.3f} cpu={r.ru_utime+r.ru_stime:.3f} rss={r.ru_maxrss/1024:.1f}MB gcs={len(gcs)} pause_ms={tot('pause_us')/1000:.1f} mark_ms={tot('mark_us')/1000:.1f} sweep_ms={tot('sweep_us')/1000:.1f} maxpause_ms={mx('pause_us')/1000:.2f} maxlive_mb={mx('live_kb')/1024:.1f} maxheap_mb={mx('heap_kb')/1024:.1f}")
if os.environ.get("SHOWGC"): print("\n".join(gcs[:int(os.environ['SHOWGC'])]))
