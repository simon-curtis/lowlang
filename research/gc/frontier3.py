import sys
sys.path.insert(0,'.')
from sim import *
alpha, beta = 100.0, 0.18
print(f"{'workload':12s} {'r':>4} | {'low ms':>7} {'peak':>5} {'avg':>4} | {'lowHW ms':>8} {'peak':>5} {'avg':>4} | {'alap':>6} {'oracle':>6} | HW/low alap/orc low/orc")
for name in sys.argv[1:]:
    A,L,T,G,K = load(f"traces/{name}.fine"); tot=A[-1]
    for r in [0.5,1,2]:
        c1,p1,g1,a1,_ = lowreal(A,L,r,4096,alpha,beta)
        c2,p2,g2,a2,_ = lowreal(A,L,r,4096,alpha,beta,hwbase=True)
        al=alap(A,L,p1,alpha,beta); o=oracle(A,L,p1,alpha,beta)
        print(f"{name:12s} {r:4} | {c1/1e3:7.1f} {p1/1024:5.0f} {a1/tot/1024:4.0f} | {c2/1e3:8.1f} {p2/1024:5.0f} {a2/tot/1024:4.0f} | {al[0]/1e3:6.1f} {o[0]/1e3:6.1f} | {c2/c1:5.2f} {al[0]/o[0]:5.2f} {c1/o[0]:5.2f}")
