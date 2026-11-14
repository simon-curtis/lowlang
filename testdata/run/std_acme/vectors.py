import base64
import hashlib
import json

from cryptography.hazmat.primitives import serialization

PEM = """-----BEGIN PRIVATE KEY-----
MIGHAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBG0wawIBAQQgyzNatCiQLkYf5er1
QdSFzno82KO6yDV7KbDdUsknkW6hRANCAARNRPNgAcXAMXmRO6/+LG1i7aN1dNgV
zLOI0SwJYJv0Q0GMVGxB/KNbnmj+OAqdSFJvqafpWJd0JWO7uHxsoq2/
-----END PRIVATE KEY-----
"""
TOKEN = "evaGxfADs6pSRb2LAv9IZf17Dt3juxGJ-PCt92wr-oA"


def b64(data: bytes) -> str:
    return base64.urlsafe_b64encode(data).rstrip(b"=").decode()


key = serialization.load_pem_private_key(PEM.encode(), password=None)
numbers = key.public_key().public_numbers()
x = b64(numbers.x.to_bytes(32, "big"))
y = b64(numbers.y.to_bytes(32, "big"))
canonical = json.dumps({"crv": "P-256", "kty": "EC", "x": x, "y": y}, separators=(",", ":"), sort_keys=True)
thumbprint = b64(hashlib.sha256(canonical.encode()).digest())
authorization = f"{TOKEN}.{thumbprint}"
print("thumbprint", thumbprint)
print("key authorization", authorization)
print("dns01 value", b64(hashlib.sha256(authorization.encode()).digest()))
