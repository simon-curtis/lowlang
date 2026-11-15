# SSH test vectors

Key files, fingerprints, certificates, `known_hosts` and `authorized_keys` files were produced by `ssh-keygen` of OpenSSH 10.6p1 (`OPENSSH_VERSION`). The private keys are test keys with no other use; `ed25519_encrypted` and `rsa_encrypted` use the passphrase `correct horse`. The `.fingerprint` files hold the output of `ssh-keygen -l`. The `*-cert.pub` files are certificates signed by `ca` with `ssh-keygen -s`; `ecdsa256_expired-cert.pub` is valid only from 2020-01-01 to 2021-01-01, and the others from 2026-01-01 to 2036-12-31. `known_hosts_hashed` was made with `ssh-keygen -H`.

`packets.txt` holds binary packets sealed by python `cryptography` 50.0.2 for `chacha20-poly1305@openssh.com`, `aes128-gcm@openssh.com`, `aes256-gcm@openssh.com`, `aes128-ctr` and `aes256-ctr` with `hmac-sha2-256-etm@openssh.com` and `hmac-sha2-512-etm@openssh.com`. One row is `cipher|mac|key|iv|mac key|sequence number|packet|payload`, all in hex. `open_packet.py` opens a packet with the same library and prints the payload.

The expected values of `testdata/run/std_bcrypt_pbkdf.low` come from python `bcrypt` 5.0.0 (`kdf`), and those of `testdata/run/std_rsa_mod_pow.low` from python's `pow`.
