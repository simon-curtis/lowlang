import sys
sys.path.insert(0, sys.argv[0].rsplit("/",1)[0])
from sim import *
alpha, beta = 100.0, 0.18
for name in sys.argv[1:]:
    A,L,T,G,K = load(f"traces/{name}.fine")
    print(f"== {name}: samples={len(A)} total_alloc_mb={A[-1]/1024:.0f} max_live_mb={max(L)/1024:.1f}")
    print(f"{'r':>5} {'peakMB':>7} {'ratio_ms':>9} {'gcs':>5} | {'alap_ms':>8} {'gcs':>5} | {'oracle_ms':>9} {'gcs':>5} | ratio/orc alap/orc")
    for r in [0.25,0.5,1,2,4]:
        c,peak,g,_ = ratio(A,L,r,4096,alpha,beta)
        al = alap(A,L,peak,alpha,beta)
        o = oracle(A,L,peak,alpha,beta)
        oc = o[0] if o else float('nan'); og = len(o[1]) if o else 0
        print(f"{r:5} {peak/1024:7.1f} {c/1e3:9.1f} {g:5} | {al[0]/1e3 if al else float('nan'):8.1f} {al[1] if al else 0:5} | {oc/1e3:9.1f} {og:5} | {c/oc:6.2f} {al[0]/oc if al else float('nan'):6.2f}")
