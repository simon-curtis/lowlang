import re,subprocess,tempfile,sys
t=open(sys.argv[1]).read()
tmp=sys.argv[2]
d=tempfile.mkdtemp(dir=tmp)
res=[]
def run(args):
    p=subprocess.run(args,capture_output=True,text=True)
    return p.returncode,(p.stdout+p.stderr).strip()
reqs=re.findall(r'(\w+) request\n(-----BEGIN CERTIFICATE REQUEST-----.*?-----END CERTIFICATE REQUEST-----)',t,re.S)
for label,pem in reqs:
    open(f'{d}/r.pem','w').write(pem+'\n')
    rc,o=run(['openssl','req','-verify','-noout','-in',f'{d}/r.pem'])
    print(f'openssl req -verify {label}: rc={rc} {o}')
blocks=re.findall(r'(\w+ (?:challenge certificate|authority|issued by \w+))\n(-----BEGIN CERTIFICATE-----.*?-----END CERTIFICATE-----)',t,re.S)
for i,(label,pem) in enumerate(blocks):
    open(f'{d}/{i}.pem','w').write(pem+'\n')
for i,(label,pem) in enumerate(blocks):
    if 'challenge' in label:
        rc,o=run(['openssl','verify','-no_check_time','-ignore_critical','-purpose','any','-CAfile',f'{d}/{i}.pem',f'{d}/{i}.pem'])
        print(f'openssl verify (self-signed, ignoring the acmeIdentifier criticality) {label}: rc={rc} {o.splitlines()[-1]}')
        rc,o=run(['openssl','x509','-noout','-text','-in',f'{d}/{i}.pem'])
        print(re.findall(r'Signature Algorithm: (\S+)',o)[0], 'SAN critical:', 'Subject Alternative Name: critical' in o, 'acmeIdentifier critical:', '1.3.6.1.5.5.7.1.31: critical' in o)
for a,b in [(4,5),(6,7)]:
    rc,o=run(['openssl','verify','-no_check_time','-CAfile',f'{d}/{a}.pem',f'{d}/{b}.pem'])
    print(f'openssl verify {blocks[b][0]} against {blocks[a][0]}: rc={rc} {o.splitlines()[-1]}')
