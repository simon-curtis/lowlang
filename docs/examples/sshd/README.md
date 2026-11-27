# An SSH server

[sshd.low](sshd.low) is an SSH server for Linux built on `std/ssh/daemon`. It reads an `sshd_config` file, `/etc/ssh/sshd_config` unless `-f` names another, and refuses one that uses a keyword or value it does not implement rather than ignoring it. `-t` checks the file and the host keys and exits.

Run as root, it checks passwords against `/etc/shadow` (yescrypt, SHA-512-crypt, SHA-256-crypt and bcrypt hashes) and public keys against each user's `authorized_keys`, starts every session as the user in a new session on a pseudo-terminal when the client asks for one, records logins in utmp and wtmp, and serves `Subsystem sftp internal-sftp` by running itself as `sshd sftp-server` under the user's account.

```
low build docs/examples/sshd/sshd.low -o /usr/local/sbin/low-sshd
/usr/local/sbin/low-sshd -t -f /etc/ssh/sshd_config
```

Run as root, it separates privilege as OpenSSH does. The daemon accepts each connection and starts itself again as `low-sshd connection`, as the `sshd` user (or `nobody`) inside `/var/empty` (or `/run/sshd`), executed through a descriptor so the empty root needs no copy of the program. That child speaks SSH to the client and asks a monitor in the daemon, over a Unix socket, to sign with a host key, check a password or a key, open a terminal and start a session. The monitor answers only for the first user the child names, counts failures against `MaxAuthTries`, verifies each key signature against the session identifier itself, builds every program from its own settings for the account it authenticated, starts nothing before authentication and kills a child that has not authenticated within `LoginGraceTime`. Run as another user, it serves every connection in one process and starts sessions only as that user.

It has no PAM, no `Match` or `Include`, no agent, X11 or remote forwarding, and a host name in a `direct-tcpip` request is not resolved inside the empty root, so local forwarding reaches addresses only.

| Program | What it does |
|---|---|
| [sshd.low](sshd.low) | Serves SSH logins, commands, terminals and SFTP as the configured `sshd_config` says |
