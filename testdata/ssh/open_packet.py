import hashlib
import hmac
import sys

from cryptography.hazmat.primitives import poly1305
from cryptography.hazmat.primitives.ciphers import Cipher, algorithms, modes
from cryptography.hazmat.primitives.ciphers.aead import AESGCM


def chacha_stream(key, counter, seq, data):
    nonce = counter.to_bytes(4, "little") + b"\0\0\0\0" + seq.to_bytes(8, "big")
    return Cipher(algorithms.ChaCha20(key, nonce), mode=None).encryptor().update(data)


def unpad(body):
    return body[1 : len(body) - body[0]]


def open_chacha(key, seq, packet):
    main_key, length_key = key[:32], key[32:]
    length = int.from_bytes(chacha_stream(length_key, 0, seq, packet[:4]), "big")
    poly_key = chacha_stream(main_key, 0, seq, bytes(32))
    poly1305.Poly1305.verify_tag(poly_key, packet[: 4 + length], packet[4 + length : 4 + length + 16])
    return unpad(chacha_stream(main_key, 1, seq, packet[4 : 4 + length]))


def open_gcm(key, iv, seq, packet):
    counter = (int.from_bytes(iv[4:], "big") + seq) & 0xFFFFFFFFFFFFFFFF
    nonce = iv[:4] + counter.to_bytes(8, "big")
    return unpad(AESGCM(key).decrypt(nonce, packet[4:], packet[:4]))


def open_ctr(key, iv, mac_key, digest, seq, packet):
    length = int.from_bytes(packet[:4], "big")
    tag = packet[4 + length :]
    expected = hmac.new(mac_key, seq.to_bytes(4, "big") + packet[: 4 + length], digest).digest()
    if not hmac.compare_digest(tag, expected):
        raise ValueError("bad mac")
    decryptor = Cipher(algorithms.AES(key), modes.CTR(iv)).decryptor()
    return unpad(decryptor.update(packet[4 : 4 + length]))


name, mac, key, iv, mac_key, seq, packet = sys.argv[1:8]
key, iv, mac_key, packet = bytes.fromhex(key), bytes.fromhex(iv), bytes.fromhex(mac_key), bytes.fromhex(packet)
seq = int(seq)
if name.startswith("chacha20"):
    payload = open_chacha(key, seq, packet)
elif "gcm" in name:
    payload = open_gcm(key, iv, seq, packet)
else:
    digest = hashlib.sha512 if "512" in mac else hashlib.sha256
    payload = open_ctr(key, iv, mac_key, digest, seq, packet)
print(payload.hex())
