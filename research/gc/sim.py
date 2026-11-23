import heapq, sys, math
def load(path):
    A=[0]; L=[0]; T=[None]; G=[0]; K=[0]
    a=0
    for l in open(path):
        if not l.startswith("gc "): continue
        d={k:int(v) for k,v in (f.split("=") for f in l.split()[2:])}
        a+=d.get("exact_kb", d["alloc_kb"]); A.append(a); L.append(d["live_kb"]); T.append(d.get("at_us")); G.append(d.get("gs",0)); K.append(d.get("stack_words",0))
    return A,L,T,G,K

def cost(L, j, alpha, beta): return alpha + beta*L[j]

def ratio(A,L,r,floor,alpha,beta):
    n=len(A); i=0; c=0.0; peak=0; gcs=0; area=0.0
    for j in range(1,n):
        if A[j]-A[i] >= max(r*L[i], floor):
            peak=max(peak, L[i]+A[j]-A[i]); area+=(L[i]+(A[j]-A[i])/2)*(A[j]-A[i])
            c+=cost(L,j,alpha,beta); gcs+=1; i=j
    peak=max(peak, L[i]+A[-1]-A[i]); area+=(L[i]+(A[-1]-A[i])/2)*(A[-1]-A[i])
    return c,peak,gcs,area

def alap(A,L,M,alpha,beta):
    n=len(A); i=0; c=0.0; gcs=0
    for j in range(1,n):
        if j+1<n and L[i]+A[j+1]-A[i] > M:
            if L[i]+A[j]-A[i] > M: return None
            c+=cost(L,j,alpha,beta); gcs+=1; i=j
    if L[i]+A[-1]-A[i] > M: return None
    return c,gcs

def oracle(A,L,M,alpha,beta):
    n=len(A); INF=float("inf"); f=[INF]*n; prev=[-1]*n
    f[0]=0.0; h=[(0.0,0)]
    for j in range(1,n):
        thr=M-A[j]
        while h and L[h[0][1]]-A[h[0][1]] > thr: heapq.heappop(h)
        if not h: return None
        best,i=h[0]
        if j==n-1:
            fin=best
            return fin, path(prev,i)
        f[j]=best+cost(L,j,alpha,beta); prev[j]=i
        heapq.heappush(h,(f[j],j))
    return None

def path(prev,i):
    p=[]
    while i>0: p.append(i); i=prev[i]
    return p[::-1]

def hw(A,L,r,floor,alpha,beta,decay=1.0):
    n=len(A); i=0; c=0.0; peak=0; gcs=0; area=0.0; H=0.0
    for j in range(1,n):
        if A[j]-A[i] >= max(r*H, r*L[i], floor):
            peak=max(peak, L[i]+A[j]-A[i]); area+=(L[i]+(A[j]-A[i])/2)*(A[j]-A[i])
            c+=cost(L,j,alpha,beta); gcs+=1; i=j
            H=max(L[j], H*decay)
    peak=max(peak, L[i]+A[-1]-A[i]); area+=(L[i]+(A[-1]-A[i])/2)*(A[-1]-A[i])
    return c,peak,gcs,area

def area_of(A,L,gcpts):
    i=0; ar=0.0; pk=0
    for j in gcpts+[len(A)-1]:
        ar+=(L[i]+(A[j]-A[i])/2)*(A[j]-A[i]); pk=max(pk,L[i]+A[j]-A[i]); i=j
    return ar,pk

def lowreal(A,L,r,floor,alpha,beta,hwbase=False):
    GROW_TIGHT_KB=3145728*8//1024
    n=len(A); i=0; c=0.0; peak=0; gcs=0; area=0.0; Hp=0; H=0
    thr=max(r*L[0],floor)
    pts=[]
    for j in range(1,n):
        occ=L[i]+A[j]-A[i]; since=A[j]-A[i]
        half=thr/2 if thr/2 < GROW_TIGHT_KB else thr/4
        if since>=thr or (occ>Hp and since>=half):
            peak=max(peak,occ); area+=(L[i]+since/2)*since; c+=cost(L,j,alpha,beta); gcs+=1; i=j; pts.append(j)
            H=max(H,L[j]); thr=max(r*(H if hwbase else L[j]),floor)
        elif occ>Hp: Hp=occ
    peak=max(peak,L[i]+A[-1]-A[i]); area+=(L[i]+(A[-1]-A[i])/2)*(A[-1]-A[i])
    return c,max(peak,Hp),gcs,area,pts

def split(A,L,half):
    mid=A[-1]/2
    if half==0:
        k=[j for j in range(len(A)) if A[j]<=mid]
        return [A[j] for j in k],[L[j] for j in k]
    k=[j for j in range(len(A)) if A[j]>=mid]
    a0=A[k[0]]
    return [A[j]-a0 for j in k],[L[j] for j in k]

def probe(A,L,M,theta,delta,b,alpha,beta,abort=True,hist=8,clear_per_kb=0.0016):
    n=len(A); i=0; c=0.0; gcs=0; aborts=0; nxt=0; good=[]
    for j in range(1,n):
        occ=L[i]+A[j]-A[i]
        if j+1<n and L[i]+A[j+1]-A[i]>M:
            c+=alpha+beta*L[j]; gcs+=1; good.append(L[j]); i=j; nxt=A[j]+delta; continue
        if occ>=theta*M and A[j]>=nxt and len(good)>=2:
            ref=sorted(good[-hist:])[len(good[-hist:])//2]
            B=b*ref
            if not abort or L[j]<=B:
                c+=alpha+beta*L[j]; gcs+=1; good.append(L[j]); i=j
            else:
                c+=alpha+beta*B+clear_per_kb*M; aborts+=1
            nxt=A[j]+delta
    return c,gcs,aborts
