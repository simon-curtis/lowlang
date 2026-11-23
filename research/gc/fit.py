import sys
# least squares fit of mark_us+sweep_us ~ a + b*live_kb + c*alloc_kb on single-worker GCs
xs=[];ys=[]
for path in sys.argv[1:]:
    for l in open(path):
        if not l.startswith("gc "): continue
        d={k:int(v) for k,v in (f.split("=") for f in l.split()[2:])}
        if d["workers"]!=1: continue
        xs.append((1,d["live_kb"],d["alloc_kb"])); ys.append((d["mark_us"],d["sweep_us"]))
import itertools
def lstsq(X,y):
    n=len(X[0]); M=[[sum(X[k][a]*X[k][b] for k in range(len(X))) for b in range(n)] for a in range(n)]; v=[sum(X[k][a]*y[k] for k in range(len(X))) for a in range(n)]
    for c in range(n):
        p=max(range(c,n),key=lambda r:abs(M[r][c])); M[c],M[p]=M[p],M[c]; v[c],v[p]=v[p],v[c]
        for r in range(n):
            if r!=c:
                f=M[r][c]/M[c][c]; M[r]=[M[r][k]-f*M[c][k] for k in range(n)]; v[r]-=f*v[c]
    return [v[c]/M[c][c] for c in range(n)]
print("n",len(xs))
print("mark  = a + b*live_kb + c*alloc_kb:", lstsq(xs,[y[0] for y in ys]))
print("sweep = a + b*live_kb + c*alloc_kb:", lstsq(xs,[y[1] for y in ys]))
