import sys, itertools
sys.path.insert(0, sys.argv[0].rsplit("/",1)[0])
from sim import *
alpha,beta=100.0,0.18
names=sys.argv[1:]
grid=list(itertools.product([0.3,0.5,0.7,0.85,0.95,1.01],[0.02,0.05,0.1,0.2],[0.3,0.45,0.6,0.8,1.0,1.2]))
print(f"{'workload':12s} {'alap':>8} {'oracle':>8} {'probe':>8} {'noabort':>8} gap_recovered probe/alap aborts params")
for n in names:
    A,L,T,G,K=load(f"traces/{n}.fine")
    _,M,_,_,_=lowreal(A,L,1,4096,alpha,beta)
    tr=split(A,L,0); te=split(A,L,1)
    def score(AA,LL,p,ab=True):
        th,dl,b=p
        return probe(AA,LL,M,th,dl*M,b,alpha,beta,abort=ab)[0]
    base_tr=alap(*tr,M,alpha,beta)[0]
    best=min(grid,key=lambda p: score(*tr,p))
    al=alap(*te,M,alpha,beta)[0]; o=oracle(*te,M,alpha,beta)[0]
    pr=probe(*te,M,best[0],best[1]*M,best[2],alpha,beta)
    na=probe(*te,M,best[0],best[1]*M,best[2],alpha,beta,abort=False)
    rec=(al-pr[0])/(al-o) if al>o else float('nan')
    print(f"{n:12s} {al/1e3:8.1f} {o/1e3:8.1f} {pr[0]/1e3:8.1f} {na[0]/1e3:8.1f} {rec:13.2f} {pr[0]/al:10.3f} {pr[2]:6d} {best}")
