import sys
sys.path.insert(0, sys.argv[0].rsplit("/",1)[0])
from sim import *
alpha, beta = 100.0, 0.18
for name in sys.argv[1:]:
    A,L,T,G,K = load(f"traces/{name}.fine")
    print(f"== {name}")
    print(f"{'r':>4} | {'ratio ms/peak/avg':>20} | {'hw ms/peak/avg':>20} | {'alap@hwpeak':>11} | {'oracle@hwpeak':>13} | hw/ratio hw/orc")
    tot=A[-1]
    for r in [0.5,1,2]:
        c1,p1,g1,a1 = ratio(A,L,r,4096,alpha,beta)
        c2,p2,g2,a2 = hw(A,L,r,4096,alpha,beta)
        al=alap(A,L,p2,alpha,beta); o=oracle(A,L,p2,alpha,beta)
        print(f"{r:4} | {c1/1e3:7.1f} {p1/1024:5.0f} {a1/tot/1024:5.0f}MB | {c2/1e3:7.1f} {p2/1024:5.0f} {a2/tot/1024:5.0f}MB | {al[0]/1e3:11.1f} | {o[0]/1e3:13.1f} | {c2/c1:5.2f} {c2/o[0]:5.2f}")
