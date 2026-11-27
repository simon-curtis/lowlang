# Standard library modules

The detail of each module in the `## Standard library` table of [design.md](design.md). The website shows a section on the module's page, under the summary from the table.

## std/sys

It is the only standard library module besides the prelude that calls `__syscall`. Its Windows files call none. It owns every syscall number, flag, errno and structure layout. It exposes typed wrappers that repeat an interrupted call and fail with an `io.Error`, so callers match `io.Error` variants.

### Descriptors

`FdHandle` is a copyable view whose closing is coordinated by an `io.Guard`, and the droppable `Fd` owns one.

`read`, `write`, `write_all`, `stream` and the `_on` forms on a view park the green thread while a non-blocking descriptor is not ready.

Further descriptor functions are `pipe`, `pipe_for_child`, `open_pipe`, `listen_pipe`, `accept_pipe`, `set_nonblocking`, `try_read`, `try_write` and `wait_input`.

| Function | Named pipe end |
| --- | --- |
| `open_pipe` | A client end of an existing Windows named pipe. |
| `listen_pipe` and `accept_pipe` | The server end: an instance of a named pipe that only the current user can open, and a wait for its client. |

All three are `Unsupported` on Linux.

### Files and processes

| Area | Functions |
| --- | --- |
| Files | `open` with an `Open` mode, `stat`, `fstat`, `access`, `chmod`, `chown`, `set_times`, `unlink`, `rmdir`, `mkdir`, `rename`, `symlink`, `link`, `readlink`, `statfs`, `lock`, `seek`, `fsync`, `ftruncate`, `list_dir`, `random_bytes`, and `memory_file` and `seal`, which make an in-memory file and seal it against changes |
| Processes | `fork_exec`, `reap`, `kill`, `kill_group`, signal dispositions and handlers, identity, working directory, `arguments`, `environment`, `executable_path`, anonymous and executable memory, `map_fork_marker` (a page that reads 0 in a forked child) and `exit_process` |

### Error numbers

| Function | Description |
| --- | --- |
| `error_message` | Turns an error number into the system's message text. |
| `os_code` | Gives the error number behind an `io.Error` (see `std/os`). |

### Memory mappings

File and anonymous mappings: `map_file` and `map_memory` take a `MapMode` (`ReadOnly`, `Shared`, `Private`) and return a `MapView`. `sync_view`, `advise_view` and `unmap_view` act on the view.

The view's offsets are a multiple of `map_granularity`, which is a page on Linux and 64 KiB on Windows.

On Windows a view comes from `CreateFileMappingW` and `MapViewOfFile`. There:

| Function | On Windows |
| --- | --- |
| `sync_view` | `FlushViewOfFile` plus `FlushFileBuffers` through a duplicate of the file handle |
| `advise_view` | `PrefetchVirtualMemory` for `WillNeed` and nothing otherwise |

### Resource limits

| Function | Description | Fails with `Unsupported` on |
| --- | --- | --- |
| `get_rlimit` and `set_rlimit` | Over `prlimit64`. | Windows |
| `job_limits` | Reads the job object of the process on Windows. | Linux |

### Sockets

Sockets: `socket`, `socketpair`, `bind`, `listen`, `accept`, `connect`, `finish_connect`, `recv`, `send`, `recv_from`, `send_to`, `write_view`, `wait_socket`, `set_option`, `get_option`, `interface_index`, `interfaces` and multicast membership.

| Function | Description |
| --- | --- |
| `recv_from_with_fd` and `send_to_with_fd` | A datagram with a descriptor passed as `SCM_RIGHTS`. |
| `recv_with_fds` and `send_with_fds` | Descriptors passed over a stream through `SCM_RIGHTS`, failing with `Unsupported` on Windows. |
| `exclusive_address` | `SO_EXCLUSIVEADDRUSE` on Windows, nothing on Linux. |

### Network interfaces

`interfaces` returns each network interface as an `Interface`. It holds the name, the kernel index, the `up`, `loopback` and `multicast` flags, and the IPv4 and IPv6 addresses. The addresses are `InterfaceAddress` values of address bytes and prefix length.

- On Linux it sends an `RTM_GETLINK` and an `RTM_GETADDR` dump request over a blocking `NETLINK_ROUTE` socket and reads the replies.
- On Linux it matches addresses to links by index, taking `IFA_LOCAL` before `IFA_ADDRESS`.
- On Linux it reads the name from `IFLA_IFNAME` and the flags from `IFF_UP`, `IFF_LOOPBACK` and `IFF_MULTICAST`.
- On Linux it fails with the error number of an `NLMSG_ERROR` reply.

On Windows it calls `GetAdaptersAddresses` for `AF_UNSPEC` with anycast, multicast and DNS server lists skipped:

- On Windows it grows the buffer while the call reports `ERROR_BUFFER_OVERFLOW`.
- On Windows it reads the adapter list and each adapter's unicast list at their documented 64-bit offsets, every pointer converted to an offset into the returned buffer and checked against its bounds.

| Field | On Windows |
| --- | --- |
| Name | The adapter's `FriendlyName` |
| Index | `IfIndex` or `Ipv6IfIndex` when that is 0 |
| `up` | An operational status of `IfOperStatusUp` |
| `loopback` | An `IfType` of 24 |
| `multicast` | The absence of `IP_ADAPTER_NO_MULTICAST` |
| Prefix length | `OnLinkPrefixLength` |

### Terminals and inotify

Terminals: `termios`, `raw_mode`, window size and `open_pty`. `inotify_init`, `inotify_add` and `inotify_remove`.

### Input log

`record_inputs(fd)` makes `record(op, name)` write one line `<op> <len> <name>` to the descriptor `fd` for each file-system and environment input the process reads. A negative `fd` stops it.

Leaving a descriptor that is logging writes `redirect 0 ` to it, so its reader knows the log is incomplete.

`record` does nothing while logging is off. It writes each line in one call under a lock and ignores write errors. The state is the module's `__static_block`.

`open`, `stat`, `access`, `statfs` and `inotify_add` (`stat`), `list_dir` (`list`), `readlink` and `fork_exec` (`exec`) record the path as passed. `std/os` records `chdir`, `env` (`getenv`) and `environment` (`environ`, with an empty name).

Writes that do not open a file, such as `unlink`, `rename`, `mkdir`, `chmod` and `set_times`, record nothing.

### Performance measurement

Measurement, which `std/perf` wraps:

| Function | Description |
| --- | --- |
| `perf_open` | Opens a `PerfEvent` on a process. |
| `perf_count` | Reads a counting one. |
| `child_usage` | Peeks at an ended child's `ChildUsage`. |
| `set_affinity` | Restricts a thread to a mask of processors. |

On Windows they fail with `Unsupported` and `child_usage` is `none`.

## std/io

- `Stream`, a copyable struct of `read`, `write` and `close` closures that views a resource.
- `Guard` and `Lease` (`guard`, `enter`, `retire`), the shared state that makes a closed descriptor fail with `NotConnected` instead of reaching a reused descriptor.
- A buffered `Reader` over a stream or, with `bytes_reader`, over a buffer, with `read_line`, `read_exact`, `read_up_to`, `read_some`, `read_byte`, `buffered` and `fill`.
- `write_all`, `read_all`, `buffer(n)`, `copy`, and the fixed-width integer codec in both byte orders.

| Operation | Functions |
| --- | --- |
| Reads as `i64` | `be16`, `be24`, `be32`, `be64` and the `le` forms |
| Reads as unsigned types | `u16_be`, `u32_be`, `u64_be` and the `_le` forms |
| Reads of any width from 1 to 8 bytes as `u64` | `uint_be` and `uint_le` |
| Work over a `str` | `u16_le_str`, `u32_le_str` and `u64_le_str` |
| Appends | `put_be16` through `put_be64`, `put_le16` through `put_le64`, and `put_be` and `put_le` for any width up to 8 bytes |
| In-place stores | `set_be16` through `set_be64` and `set_le16` through `set_le64` |

## std/file

| Receiver | Functions |
| --- | --- |
| On a `File` | `read`, `write`, `write_all`, `seek`, `sync`, `truncate`, `close`, `info` and `stat` |
| On a `FileHandle` | `read_on`, `write_on`, `write_all_on`, `close_on` and `stream_on` |
| Whole files and directories | `exists`, `read_file`, `read_text`, `write_file`, `write_text`, `remove`, `remove_dir`, `rename`, `mkdir`, `mkdir_all`, `list` and `stream` |

### Reading whole files

`read_file` sizes its buffer from the file's size. A file that reports no size, such as one under `/proc`, is read into a 64 KB buffer that doubles when full.

### Permission bits

Permission bits: `create_mode`, `write_file_mode`, `write_text_mode`, `mkdir_mode` and `mkdir_all_mode` take them.

| Creators | Permission bits |
| --- | --- |
| `create_mode`, `write_file_mode`, `write_text_mode`, `mkdir_mode` and `mkdir_all_mode` | Taken as an argument |
| The other creators | `0o644` for files and `0o755` for directories |

The process umask filters all of them. `chmod` is not filtered.

### Locks and removal

| Function | Description |
| --- | --- |
| `try_lock` and `unlock` | Advisory `flock` locks that report a held lock as `false` instead of waiting. |
| `remove_all` | Removes a tree without following symbolic links. |

### Temporary files and directories

| Function | Description |
| --- | --- |
| `temp_dir` | Creates a private directory under `os.temp_dir_path()`, which is `TMPDIR` when set and `/tmp` otherwise. |
| `temp_file(dir, pattern)` | Creates a uniquely named file with `O_EXCL` and mode `0600` and returns a droppable `TempFile` that closes and removes it unless `keep` is called. |
| `temp_dir_pattern` | Makes a `0700` directory the same way. |

`temp_file` and `temp_dir_pattern` use that directory when `dir` is empty. All three draw the random name suffix from `getrandom`.

### Paths and separators

`child_path` joins a directory and an entry name.

| Function | On Windows |
| --- | --- |
| `mkdir_all`, `child_path` and the `temp_file` and `temp_dir_pattern` name check | Treat `\` as a separator as well as `/`. |
| `mkdir_all` | Never creates a drive (`C:\`), share (`\\server\share\`) or verbatim (`\\?\C:\`) prefix. |

The separator and volume rules are `std/filepath`'s. `copy` and `walk` also use them to take apart the paths they are given.

### More file functions

`Info` also carries `regular`, `link`, `accessed`, `uid`, `gid`, `device` and `inode`.

Further functions: `lstat`, `is_dir`, `is_file`, `is_symlink`, `symlink`, `link`, `readlink`, `realpath`, `chown`, `lchown`, `set_times`, `disk_usage` and `read_dir_info`.

| Function | Description |
| --- | --- |
| `walk` | A sorted depth-first walk that uses `lstat`, never descends into symbolic links and lets its callback return `Continue`, `Skip` or `Stop`. |
| `glob` | Matches each segment with `path.matches` and supports `**`. |
| `copy` and `copy_dir` | Keep file modes and recreate symbolic links. |

### Memory mappings

| Function | Description |
| --- | --- |
| `map` | Maps a range of an open file. |
| `map_anonymous` | Maps a region of zeros. |

Each is given a `MapMode` (`ReadOnly`, `Shared` or `Private` copy-on-write) and returns a droppable `Mapping`. Its `data` is a `u8[]` over memory outside the collector's heap, so it is neither scanned nor moved and stays valid until the `Mapping` is dropped or `unmap` releases it. A copy of `data` used afterwards panics with `invalid memory access`. So does a write to a `ReadOnly` mapping. A `push` onto a copy reallocates on the heap.

| Range | Behaviour |
| --- | --- |
| `offset` | Need not be aligned, because the mapping starts at the enclosing page (Linux) or 64 KiB boundary (Windows), and `data` begins at `offset`. |
| A `length` of `0` | Maps to the end of the file. |
| A range that leaves the file | `Invalid`. |
| An empty range | Gives an empty `data` with nothing to release. |

| Function | Takes | Uses |
| --- | --- | --- |
| `sync_map` | `Wait` or `Queue` | `msync`, or on Windows `FlushViewOfFile` and, for `Wait`, `FlushFileBuffers` |
| `advise_map` | `Normal`, `Random`, `Sequential`, `WillNeed` or `DontNeed` | `madvise`, or on Windows `PrefetchVirtualMemory` for `WillNeed` and nothing for the rest |

## std/file/watch

`watch` and `watch_with` take paths and `Options` (`recursive`, `access`) and return a droppable `Watcher` with a copyable `WatcherHandle`.

On a watcher: `add`, `remove`, `next`, `events` and `stop`.

| Function | Description |
| --- | --- |
| `next` | Parks the green thread until an `Event` (`watched`, `name`, `path`, `kind`, `dir`, `cookie`) arrives. |
| `events` | A `chan[Event]` fed by a forwarding green thread. |

`Kind` is `Created`, `Modified`, `Deleted`, `MovedFrom`, `MovedTo`, `Attrib`, `Closed`, `ClosedUnmodified`, `Opened`, `Accessed`, `SelfDeleted`, `SelfMoved` or `Overflow`.

## std/net

Types: the droppable `Listener`, `Conn` and `Socket`, each with a copyable `ListenerHandle`, `ConnHandle` or `SocketHandle` in its `handle` field.

| Function | Description |
| --- | --- |
| `adopt` and `detach` | Convert between a `ConnHandle` and a `Conn`. |
| `adopt_socket` and `detach_socket` | Convert between a `SocketHandle` and a `Socket`. |
| `inherit` | Makes a `Conn` of a socket descriptor the process was started with. |

### TCP

TCP: `listen`, `accept`, `connect`, `dial` and their `_within` forms with a connect timeout, `interface_index` and `interfaces`.

| Function | Description |
| --- | --- |
| `accept_handle` | Returns the connection's view for a green thread to adopt. |
| `connect_through` | Binds the socket to a network interface first. |
| `connect_first` | Tries a list of addresses in order through an optional interface. |

### Network interfaces

`interfaces` lists the machine's network interfaces as `Interface` values holding `name`, `index`, `up`, `loopback`, `multicast` and `addresses`. It fails with an `io.Error` when the system's tables cannot be read. Each address is an `InterfaceAddr` of the same `ip` bytes `parse_ip` returns and a `prefix` length.

### Connection I/O

| Where | Functions |
| --- | --- |
| On a connection | `read`, `write`, `write_all`, `close`, `shutdown_write`, `set_nodelay`, read and write timeouts and `stream` |
| On the view | `read_on`, `write_on`, `write_all_on`, `close_on` and `stream_on` |

### UDP

| Where | Functions |
| --- | --- |
| UDP | `bind_udp`, `send_to`, `receive_from` and `close_socket` |
| On the view | `send_to_on`, `receive_from_on` and `close_socket_on` |

A `SocketHandle` has separate `fd` and `out` descriptors, so one green thread can wait to receive while another waits to send, and closing the socket fails a receive that is waiting with `NotConnected`.

### Unix datagram sockets

Unix datagram sockets use the same functions.

| Function | Description |
| --- | --- |
| `bind_unix_datagram` | Binds a pathname or abstract name and unlinks the file when closed. |
| `unix_datagram` | Makes a sender with no name, whose datagrams `receive_from` reports with an empty `Addr`. |
| `send_to_with_fd` and `receive_from_with_fd` (with `_on` forms) | Pass a descriptor along with a datagram. |

### Name lookup

Names: `lookup` returns the addresses of a host through `resolve` with `system_resolver`, so it tries an IP literal, the hosts file, `localhost` and then DNS. It fails with `NotFound` when the name does not exist or has no address.

A `Resolver` carries the servers, search list, `ndots`, timeout, attempts, rotation, EDNS(0) and TCP-only settings, an optional `cancel.Token` and the `pick` function `lookup_srv` draws from.

| Function | Description |
| --- | --- |
| `resolver` | Takes a `ResolverConfig` from `resolver_config`. |
| `system_resolver` | Reads `system_config`. |
| `resolve` | Returns `dns.Error` (`NoSuchName`, `NoData`, `ServerFailure`, `FormatError`, `NotImplemented`, `Refused` or `Io`). |

`lookup_mx`, `lookup_txt`, `lookup_srv`, `lookup_caa`, `lookup_ptr`, `lookup_cname` and `lookup_ns` return typed records, with MX ordered by preference and SRV by priority and a weighted shuffle (RFC 2782).

### Resolver configuration

`parse_resolv_conf` reads `nameserver` (up to three, IPv4 or IPv6), `search`, `domain` and `options ndots timeout attempts rotate edns0 use-vc`. `LOCALDOMAIN` and `RES_OPTIONS` override the file.

The search list follows `ndots`:

| Name | Tried |
| --- | --- |
| A name with at least that many dots | As given first |
| A name with a trailing dot | Only as given |

| Target | Reads |
| --- | --- |
| The Linux target | `/etc/resolv.conf` and `/etc/hosts` |
| The Windows target | Up to three servers and the primary domain from `GetNetworkParams`, and the hosts file under the system directory |

`std/net/dns` holds the message codec.

### Unix addresses

An `Addr` with an empty `ip` is a Unix address. Abstract names start with `@` or NUL. A listener unlinks its pathname when closed.

| Function | Description |
| --- | --- |
| `listen_unix`, `connect_unix`, `connect_unix_within` and `socketpair` | Return the same `Listener` and `Conn`. |
| `peer_credentials` | Returns the peer's credentials. |
| `read_with_fds` and `read_with_fds_on` | Return an `FdRead` (the byte count and the descriptors that arrived with them). |
| `write_with_fds` and `write_with_fds_on` | Attach descriptors to a write. |

The caller closes the received descriptors with `sys.close_fd`.

### Listen options

`listen_with` and `bind_udp_with` take `ListenOptions` (`reuse_addr`, `reuse_port`, `backlog`). `listen` sets `reuse_addr` and `bind_udp` does not, so no two UDP sockets share a port unless both ask for it.

### Socket options and multicast

| Area | Functions |
| --- | --- |
| Socket options | `set_keepalive` and `set_buffer_sizes` |
| UDP multicast | `join_group`, `leave_group`, `set_multicast_ttl`, `set_multicast_loop` and `set_multicast_interface` |

### Dialers

| Name | Description |
| --- | --- |
| `Dialer` | A function value that makes a connection. |
| `direct_dialer` | The plain one. |

## std/net/sntp

`request` writes the 48-byte version 4 client packet.

`parse` checks a reply against the `Stamp` that was sent. It checks, in order:

1. the length
2. mode 4
3. version
4. the originate timestamp
5. kiss-o'-death (stratum 0, with the code as `KissOfDeath`)
6. a leap indicator of 3
7. stratum 1 to 15
8. a nonzero transmit timestamp

It then returns a `Reply` with the offset and delay in nanoseconds:

| Value | Formula |
| --- | --- |
| Offset | `((t2 - t1) + (t3 - t4)) / 2` |
| Delay | `(t4 - t1) - (t3 - t2)` |

A `Stamp` is the 32-bit seconds and 32-bit fraction of an NTP timestamp. `to_stamp` and `to_unix_nanos` convert it with the era rule of section 3, so seconds with the top bit clear count from 2036-02-07T06:28:16Z, for times from 1968 to 2104.

`query` resolves a host and sends one request from its own UDP socket, with the low 8 bits of the transmit stamp random. It ignores datagrams from other sources and tries the next address when one fails with `Io`.

`Options` sets the port, an overall timeout, the clock that supplies `t1` and `t4`, and a cancellation token that ends the wait with `Io(Aborted)`.

## std/net/punycode

`decode` reads them back. Both fail with `Error`: `Overflow`, `InvalidUtf8`, `BadDigit`, `Truncated` or `InvalidCodePoint`.

## std/net/idna

`to_unicode(domain, Options)` returns an `Unicode` holding the converted text and the first error. `Options` holds `check_hyphens`, `check_bidi`, `check_joiners`, `use_std3_ascii_rules` and `verify_dns_length`, all `true` by default, and `transitional`, `false` by default.

## std/net/publicsuffix

`registrable_domain` returns the eTLD+1 and `is_public_suffix` returns a `bool`. All three take `Rules.All` or `Rules.Icann`. Names go in and come back in the form the caller wrote: ASCII, `xn--` or Unicode. They match through `std/net/idna`. `list_version` and `list_commit` report the embedded list.

## std/quic

`Config` holds:

- the `tls.Config` (ALPN required)
- idle and handshake timeouts
- keep-alive
- the flow control and stream limits advertised to the peer
- `retry`

| Function | Description |
| --- | --- |
| Server: `listen` | Returns a droppable `Listener` with a copyable `ListenerHandle`. |
| Server: `accept` and `accept_handle` | Wait for an established connection. |
| Client: `dial`, `dial_handle`, `connect` and `connect_handle` | Complete the handshake before returning. |

Server: `close_listener` and `close_listener_on`.

The droppable `Connection` closes with code 0 when released. Its `*ConnectionHandle` holds the negotiated `protocol`, the addresses, `group`, `suite`, `peer_certificates` and, once ended, `close_code`, `close_reason`, `closed_by_peer` and `application_close`.

`adopt`, `detach`, `close_connection`, `close_on`, `stats` and `stats_on` (a `Stats` snapshot), and `update_keys` and `update_keys_on`.

| Function | Description |
| --- | --- |
| Streams: `open_stream`, `open_uni_stream`, `accept_stream` and `accept_uni_stream` with `_on` forms | Return a copyable `Stream`. |
| `read`, `write`, `finish`, `reset`, `stop` and `stream` | `stream` adapts it to an `io.Stream`. |

## std/quic/wire

- A bounds-checked `Cursor` with varints and packet numbers
- `parse_header` for long and short headers
- `version_negotiation`
- packet protection (`keys`, `initial_keys`, `next_keys`, `seal`, `open`, `peek_key_phase`) with header protection
- Retry packets and their integrity tag (`retry_packet`, `retry_tag`, `retry_valid`)
- the frames as one `Frame` enum (`parse_frames`, `put_frame`, `frame_size`, `put_stream_frame`, `ack_eliciting`, `ack_delay_ns`)
- transport parameters (`encode_parameters`, `decode_parameters`)
- the transport error codes

Every parser fails `Malformed` on hostile input and never panics.

## std/quic/recovery

It keeps the RTT estimate, declares packets lost by packet and time thresholds, runs the probe timeout and NewReno with persistent congestion, and answers `can_send`, `deadline` and `pto_duration`. `skip` records a packet number left unsent, and an ACK covering one fails `Malformed`.

## std/net/dns

`Message` holds:

- the header flags
- the 12-bit `rcode` (the extended bits come from the OPT record)
- `questions`
- the answer, authority and additional `Record`s

A `Record`'s `data` is a `Data`: `A`, `Aaaa`, `Ns`, `Cname`, `Ptr`, `MxData`, `Txt`, `SrvData`, `SoaData`, `CaaData`, `Opt` (with `EdnsOption`s) or `Raw` for a type it does not know, which keeps its bytes. In an OPT record the class is the UDP payload size and the ttl is the version and flags.

| Function | Description |
| --- | --- |
| `encode` | Writes a message with name compression and fails `Invalid` or `TooLarge` for a label over 63 bytes, a name over 255 octets or a field that does not fit. SRV targets are not compressed. |
| `decode` | Follows compression pointers that point strictly backwards, at most 127 per name. It fails `Malformed` on a loop, a forward pointer, a reserved label type, a bad length or a record whose data does not fill its length. |
| `decode_head` | Reads the header and questions only. |
| `query` | Builds a recursive query with an OPT record when the payload size is positive. |
| `reverse_name` | Returns the `in-addr.arpa` or `ip6.arpa` name of an address. |
| `same_name` | Compares names without ASCII case. |
| `order_mx` | Sorts by preference. |
| `order_srv` | Applies the RFC 2782 weighted selection through a `pick` function. |

A decoded message shares no memory with its input. Trailing bytes are ignored.

Names are strings without a trailing dot (`.` for the root) with `\.`, `\\` and `\DDD` escapes.

`Error` is `NoSuchName`, `NoData`, `ServerFailure`, `FormatError`, `NotImplemented`, `Refused` or `Io`, and the type, class and rcode numbers are constants.

## std/socks

The proxy resolves the target name.

## std/ssh

`client`, `dial`, `dial_through` (ProxyJump) and `serve_stream`, `serve`, `serve_on` return a droppable `Connection` whose `handle` is the copyable `*ConnectionHandle`. Dropping it stops the reader and fails pending requests.

| Type | Description |
| --- | --- |
| `ClientConfig` | Carries the user, the host key callback, the `AuthMethod` list (`NoAuth`, `Password`, `PublicKey`, `KeyboardInteractive`), the rekey limits and `handshake_timeout`. |
| `ServerConfig` | Carries the host keys, `ServerAuth` callbacks, the channel and global request handlers and the same limits. |
| `Channel` | A droppable `Channel` has `read_on`, `write_on`, `write_stderr_on`, `close_write_on`, `stream` and `wait_exit_on`. |

| Area | Functions |
| --- | --- |
| Sessions | `open_session`, `open_session_on`, `exec`, `exec_on`, `shell_on`, `subsystem_on`, `open_subsystem`, `pty_on`, `env_on`, `window_change_on`, `signal_on`, and on the server `next_request_on`, `answer_on`, `parse_exec`, `parse_pty`, `parse_env`, `send_exit_status_on` and `send_exit_signal_on` |
| Forwarding | `direct_tcpip`, `direct_tcpip_on`, `direct_streamlocal`, `listen_local`, `listen_remote`, `accept_forward`, `cancel_forward`, `relay`, `serve_direct_tcpip` and `forward_global` |

`keepalive`, `key_exchanges`, `wait_on` and `disconnect_on` complete the connection operations.

| Server function | Description |
| --- | --- |
| `accept_on` | Accepts from a listener and calls a handler per connection with a function to call once it has authenticated, closing connections as `ServerConfig.startups` (OpenSSH's `MaxStartups`) says. |
| `serve_from` | Serves one connection over a stream with its local and remote addresses, which the `ConnectionHandle` and `connection_of(incoming)` give the handlers. |
| `check_signature` | Checks a `KeySignature`, a public key login's signature over the session identifier. `ServerAuth.signed_key` defaults to it, so a server can have another process decide. |

`ServerConfig.login_grace` closes a connection that has not authenticated within that many seconds.

The `*ConnectionHandle` is the thread-safe, copyable view of a live connection. `spawn` takes it where it refuses the droppable `Connection`. `open_session_on`, `direct_tcpip_on` and `keepalive_on` run on it from any green thread, so a program can run its own accept loop and open one `direct-tcpip` channel per client; global requests take turns on the connection, channel opens are matched to their replies by channel number, and once the connection ends all three fail instead of hanging.

## std/ssh/wire

A `Reader` (`reader`, `read_u8`, `read_bool`, `read_u32`, `read_u64`, `read_string`, `read_str`, `read_mpint`, `read_names`, `read_bytes`, `left`, `offset`, `rest`, `finish`) that fails with `Malformed` on short, negative or non-minimal input, and the `put_*` writers `put_u32`, `put_u64`, `put_bool`, `put_string`, `put_str`, `put_mpint` and `put_names`, with `has_name`.

## std/ssh/key

- Public key lines (`parse_line`, `format_line`, `fingerprint`)
- `authorized_keys` with options (`parse_authorized_keys`, `find_authorized`, `authorized_option`)
- `known_hosts` with hashed names, `@cert-authority` and `@revoked` (`parse_known_hosts`, `check_host_key`, `known_hosts_verifier`, `trust_on_first_use`)
- `openssh-key-v1` private keys with `aes256-ctr` and bcrypt KDF encryption and PEM keys through `std/crypto/x509` (`load_private_key`, `parse_private_key`)
- `Signer`, `sign`, `verify`
- and certificates (`parse_certificate`, `certificate_signed_by`, `check_certificate`, `certified_signer`)

## std/ssh/agent

`connect`, `connect_env` (`SSH_AUTH_SOCK`, and on Windows `\\.\pipe\openssh-ssh-agent` when it is unset), `connect_pageant` (Pageant through its window, on Windows), `connect_any` (`SSH_AUTH_SOCK`, then on Windows the OpenSSH pipe and Pageant), `identities`, `sign` and `signers`, which returns `key.Signer`s that `AuthMethod.PublicKey` takes.

## std/ssh/sftp

`connect` over a stream and `open` over an SSH connection return a droppable `Client` whose `handle` is the `*Session` every operation takes.

The operations are `stat`, `lstat`, `set_attrs`, `remove`, `mkdir`, `rmdir`, `real_path`, `rename`, `read_link`, `symlink`, `list`, `open_dir`, `read_dir`, `read_file`, `write_file`, and the OpenSSH extensions `posix_rename`, `hardlink`, `statvfs` and `limits`.

`open_file` returns a droppable `File` with `read`, `read_at`, `write`, `write_at`, `seek`, `fstat`, `set_file_attrs`, `sync`, `flush`, `close` and `stream`, which gives an `io.Stream`.

| Function | Description |
| --- | --- |
| `serve` | Runs the server over a stream. |
| `handler` | Returns the channel handler that serves the `sftp` subsystem for `ssh.ServerConfig`. |
| `serve_from` | Runs the server over a stream with relative paths taken from a start directory inside the root, as a login's home directory. |

All three are confined to a root directory.

## std/ssh/login

`handler(Config)` returns the channel handler that serves `session` channels as `sshd` does: `pty-req`, `env`, `shell`, `exec`, `subsystem sftp`, `window-change` and `signal`. `Config` says how an `Account` is found (`system_account` reads `/etc/passwd` and `/etc/group`), which variables `accept_env` admits as patterns, whether terminals are offered, the `PATH`, the `sftp` command, and the `Launcher`.

| Function | Description |
| --- | --- |
| `prepare` | Turns a `Request` (a `Program` of `Shell`, `Command` or `Subsystem`, the variables, the terminal) into the `Launch` of the account's login shell, `shell -c` or the sftp command, or `none`. |
| `local` | The `Launcher` that opens terminals with `open_terminal_for` and starts each `Launch` with `start_launch` in this process, as the account when it differs from the process's user. |
| `records` | Writes each terminal session's start and end to a utmp and a wtmp file. |

A session on a terminal leads a new session whose controlling terminal it is. Without one its streams are pipes and standard error is the channel's extended data. The channel gets `exit-status` or `exit-signal` when the child ends, and closing it first hangs up the child's process group.

## std/ssh/daemon

`parse_config` reads an `sshd_config` file into `Settings` and fails on any keyword or value it does not implement. `prepare` loads the host keys into a `Daemon`, `server_config` builds its `ssh.ServerConfig` and `serve` listens on every `Port` and `ListenAddress`.

| Function | Description |
| --- | --- |
| `check_password` | Checks a password against the user's `/etc/shadow` entry and the root and user policies. |
| `authorized` | Checks a public key against each `AuthorizedKeysFile`, under `StrictModes` through `safe_path`, and refuses an entry whose options it does not enforce. |
| `permitted` | Applies `AllowUsers`, `DenyUsers`, `AllowGroups` and `DenyGroups`. |
| `separation_for` | Returns the `Separation` of the `sshd` or `nobody` user and `/var/empty` or `/run/sshd`. |
| `serve_separated` | Serves one connection as the unprivileged child of a separated daemon. |
| `serve_sftp` | Serves SFTP on standard input and output from the working directory, for `Subsystem sftp internal-sftp`. |

With a `Separation`, `serve` starts the daemon's executable again for each connection as the unprivileged user inside the empty root. That child speaks SSH and asks a monitor in the daemon, over a Unix socket that carries descriptors, to sign with a host key, check a password or key, open a terminal and start or signal a session. The monitor signs the session identifier first and only hashes after it, answers for one user, counts failures, checks key signatures itself, builds each program from its own settings for the authenticated account and starts nothing before authentication.

## std/mdns

`query` makes a one-shot query, `answer` reads the address a response gives for a name, and `respond` answers a query for a list of `Host`s as a `Reply` to the querier or the group. `group`, `port`, `is_local` and `random_name` (a version 4 UUID under `.local`) complete the codec. `resolve` sends the queries from port 5353 and waits for an address, and `responder` returns a droppable `Responder` that answers for the names its `Handle` is given by `publish`.

## std/stun

`Message` (class, method, 12-byte transaction, attributes), `message`, `add`, `find`, `encode`, `parse` and `looks_like` (the RFC 7983 demultiplexing test).

| Function | Description |
| --- | --- |
| `seal` | Appends MESSAGE-INTEGRITY (HMAC-SHA1), MESSAGE-INTEGRITY-SHA256 and FINGERPRINT as a `Protection` asks. |
| `check_integrity` and `check_fingerprint` | Verify a parsed message. |
| `client` | Binds a UDP socket and a reader and returns a droppable `Client`. |
| `request` | Sends a message with the retransmission schedule of `waits` and returns the matching response. |
| `binding_address` | Asks a server what address it sees. |
| `respond` | Answers one datagram for a Binding server, with short-term credentials. |
| `serve` | Runs it on a socket. |

Typed attributes: `add_address`, `add_xor_address`, `get_address`, `get_xor_address`, `reflexive_address`, `add_text`, `get_text`, `add_u32`, `get_u32`, `add_u64`, `get_u64`, `add_error_code`, `error_code`, `add_unknown_attributes` and `unknown_required`, with a constant for every method and every attribute ICE and TURN use.

Long-term keys come from `long_term_key` (MD5) and `long_term_key_sha256`.

## std/ice

`Candidate`, `Kind`, `priority`, `pair_priority`, `foundation`, `local_preference`, and `format` and `parse` for the SDP `candidate` attribute.

`agent` makes an `*Agent` from a `Config`.

- The caller gives it received datagrams with `receive` and the time with `tick`.
- The caller drains `poll_transmit` and `poll_event`, and `next_timeout` says when `tick` is next due.

| Function | Description |
| --- | --- |
| `gather`, `set_remote_credentials`, `add_remote_candidate`, `end_of_remote_candidates`, `set_remote_lite`, `restart`, `selected`, `note_sent` and `close` | Drive it. |
| `receive_relay` and `relay_lost` | Carry the messages and the end of a TCP or TLS connection to a TURN server. |
| `local_addresses` | Lists the host's interface addresses. |

## std/turn

| Function | Description |
| --- | --- |
| `client` | Makes a `*Client` from a `Config` (the server, the `Transport` and TLS server name, the long-term username and password, the lifetime asked for and the retransmission schedule). |
| `start` | Sends the Allocate. |
| `receive` | Takes each message from the server and returns the `Payload` a peer sent. |
| `send_to` | Relays data to a peer. |
| `tick`, `next_timeout`, `poll_transmit`, `poll_event` (an `Event` when the allocation succeeds or fails) and `close` | Drive it. |
| `connection_lost` | Ends an allocation whose connection closed. |
| `Client.state` and `Client.relayed` | Report the allocation. |
| `ChannelData`, `is_channel_data`, `encode_channel_data` and `parse_channel_data` | Frame channel data. |
| `Framer`, `framer`, `feed` and `partial` | Split a TCP or TLS stream into messages. |
| `parse_uri` | Reads a `turn:` or `turns:` URI into a `Uri`. |

## std/ice/session

`Options.tls` and `Options.connect_timeout` set up the connections to TCP and TLS TURN servers. `Session` is the droppable owner and `Handle` its copyable view, which `credentials`, `gather`, `candidates`, `set_remote_credentials`, `add_remote_candidate`, `end_of_remote_candidates`, `set_remote_lite`, `restart`, `send`, `selected`, `state` and `close_session` take. `Handle.events` carries the agent's events and `Handle.incoming` the datagrams that are not STUN.

## std/dtls

`Config` names:

- the role
- the `Identity`
- the peer `Fingerprint`s to accept (or `unverified`)
- the cipher suites, groups and `use_srtp` profiles
- the versions from `min_version` to `max_version` (`dtls12` and `dtls13`)

`conn` makes a `*Conn`.

- The caller gives it received datagrams with `receive` and the time with `tick`.
- The caller drains `poll_transmit`, `poll_event` and `read`, and `next_timeout` says when `tick` is next due.

| Function | Description |
| --- | --- |
| `start` | Sends the first ClientHello. |
| `write` | Sends one application message. |
| `close` | Sends `close_notify`. |
| `update_keys` | Sends a DTLS 1.3 KeyUpdate. |
| `state`, `version`, `peer_certificate`, `cipher_suite` and `srtp_profile` | Report the connection. |
| `export_keying_material` and `export_keying_material_context` | Return the keying material of RFC 5705 or, over DTLS 1.3, of RFC 8446 section 7.5. |
| `generate_identity` | Makes a self-signed ECDSA P-256 certificate. |
| `fingerprint`, `format_fingerprint`, `parse_fingerprint` and `fingerprint_matches` | Handle the SDP `a=fingerprint` digests. |

## std/dtls/session

`open` returns a droppable `Session` that sends `close_notify` when released, and `Handle` is its copyable view, which `wait_connected`, `send`, `receive`, `update_keys`, `export_keying_material`, `describe_session` and `close_session` take. `describe_session` returns an `Info` that includes the version. `Handle.data` carries the application messages and `Handle.events` the connection's events.

## std/rtp

`parse` and `marshal` read and write a `Packet` with its contributing sources, padding and header `Extension`s in the one-byte or two-byte form of RFC 8285.

| Function | What it does |
| --- | --- |
| `header_length`, `ssrc_of`, `sequence_of` | Read a header without parsing the rest. |
| `find_extension` | Looks an element up. |
| `sequence_after` | Compares sequence numbers across the wrap. |
| `parse_rtcp`, `marshal_rtcp` | Read and write a compound packet as `Rtcp` values: `SenderReport`, `ReceiverReport` with `ReportBlock`s, `SourceDescription`, `Goodbye`, `GenericNack`, `PictureLoss`, any other feedback as `FeedbackMessage` and any other type as `RawPacket`. |
| `classify` | Names what a datagram on a shared WebRTC socket is (RFC 7983). |

## std/srtp

`context` keys a `*Context` for one direction from a master key and salt. `from_dtls` turns the `EXTRACTOR-dtls_srtp` keying material of a DTLS handshake into the send and receive `Pair` of one endpoint. `keying_material_size` says how much to export. `protect` and `unprotect` handle RTP, and `protect_rtcp` and `unprotect_rtcp` handle RTCP. Every failure is an `Error`.

## std/sctp

| Name | What it does |
| --- | --- |
| `Config` | Names the role, the ports, the packet size, the receiver window and the timers. |
| `assoc` | Makes an `*Assoc`. |
| `receive` | Is how the caller gives the `*Assoc` received packets. |
| `tick` | Is how the caller gives the `*Assoc` the time. |
| `poll_transmit`, `poll_event`, `read` | The caller drains them. |
| `next_timeout` | Says when `tick` is next due. |
| `connect` | Sends the INIT. |
| `send` | Queues one message on a stream with a `Delivery`. |
| `reset_stream` | Asks the peer to restart a stream's sequence numbers. |
| `shutdown`, `abort` | End the association. |
| `state`, `buffered_amount` | Report the association. |

## std/sctp/session

`open` returns a droppable `Session` and `Handle` is its copyable view, which `wait_connected`, `send`, `receive`, `reset_stream`, `buffered_amount`, `shutdown_session` and `close_session` take. `Handle.messages` carries the messages the peer sent and `Handle.events` the association's events.

## std/datachannel

`open` takes an `sctp.Config`, whose `client` is true for the DTLS client, and a `session.Transport`. It returns a droppable `Peer` whose `Handle` `wait_connected`, `open_channel`, `shutdown_peer` and `close_peer` take.

`open_channel` takes `Options` (label, protocol, ordered, `max_retransmits` or `max_lifetime`, priority, or `negotiated` with an `id`) and returns a `Channel`. `send_text`, `send_binary`, `receive` and `close_channel` use it.

| Name | What it does |
| --- | --- |
| `Handle.incoming` | Carries the channels the peer opened. |
| `Channel.messages` | Carries the messages. |
| `Open`, `encode_open`, `parse_open`, `encode_ack`, `is_ack` | The DCEP codec. |

## std/sdp

`parse` reads a `Session` of `Origin`, `Connection`s, `Bandwidth`s, `Timing`s and `Media` with their `Attribute`s, and `format` writes it with CRLF line ends. The accessors take the `Session` and the `Media`.

| Function | What it does |
| --- | --- |
| `ice_ufrag`, `ice_pwd`, `ice_options`, `fingerprints`, `setup`, `direction`, `end_of_candidates` | Read an attribute from the media or else the session. |
| `mid`, `candidates`, `sctp_port`, `max_message_size`, `rtpmaps`, `fmtps`, `feedbacks`, `extmaps`, `msids`, `ssrcs`, `ssrc_groups`, `rtcp_mux`, `rtcp_rsize` | Read the media alone. |
| `groups`, `bundle` | Read the session. |
| `rtpmap_attribute`, `fmtp_attribute`, `extmap_attribute`, `group_attribute`, `setup_attribute`, `direction_attribute`, `msid_attribute`, `ssrc_attribute`, `ssrc_group_attribute` | Build attributes to put in a description. |

## std/webrtc

`open` takes a `Config` and returns a droppable `Peer` whose `Handle` the other functions take.

`Handle.events` carries each `Event` (local candidates, the `State`, the selected ICE pair, and a `RemoteTrack` for each track the remote sends, whose `packets` carry its RTP). `Handle.channels` carries the data channels the remote opens and `Handle.rtcp` its RTCP.

| Function | What it does |
| --- | --- |
| `add_track` | Adds a local `Track` of a `Kind` with its `Codec`s (`opus` and `vp8` give the browser defaults). |
| `create_offer`, `create_answer`, `apply_answer` | Exchange `sdp.Session`s, as many times as the peers renegotiate. |
| `rollback` | Is for crossed offers. |
| `signaling` | Is for the `Signaling` state. |
| `restart_ice` | Is for an ICE restart. |
| `add_remote_candidate`, `end_of_remote_candidates` | Take trickled candidates. |
| `transceivers` | Lists the media sections as `Transceiver`s. |
| `set_direction`, `stop_section` | Change one by its `mid`. |
| `remove_track` | Stops a track sending. |
| `wait_connected`, `create_data_channel`, `codecs`, `send_rtp`, `send_rtcp`, `close_peer` | Complete the module. |

`Error` says why a description was refused.

## std/pipe

A `Pipe` owns a `Reader` and a `Writer`, each droppable so that releasing it completes it, with copyable `ReaderHandle` and `WriterHandle` views for green threads (`adopt_reader`, `adopt_writer` and the `_on` operations). Also segmented buffers, backpressure, `Sequence` and `SequenceReader`, and adapters between pipes and streams.

## std/cancel

`on_cancel` returns a droppable `Registration` that releases its callback. `release`, `on_cancel_handle` and `release_on` work on the copyable `RegistrationHandle`. `pause` is a sleep that ends early on cancellation.

## std/header

`Header`, `get`, `values`, `has`, `add`, `set`, `remove`, `token`, `token_byte`, `valid_value`, `valid`, `quote`, and `parse_value` with `param`, which resolve RFC 2231 and 5987 parameters.

## std/http

### HTTP request fields

| Field | What it is |
| --- | --- |
| `Request.token` | Is cancelled when the client goes away and when the handler returns. |
| `Request.remote` | The client's IP address for connections served by `serve`, `serve_tls` and `serve_conn`. |

### HTTP serving

Serving: `serve`, `serve_tls`, `serve_conn` and `serve_stream`, and `serve_quic` for HTTP/3 over a `std/quic` listener. The `_on` forms (`serve_on`, `serve_tls_on`, `serve_conn_on`, `serve_quic_on`) take the listener's or connection's view.

### HTTP client

Client: a pooled client configured through `client()` and `with_*` builders, per-request builders, `send`, `stream_to` and `download`.

| Builder | What it does |
| --- | --- |
| `with_version(Http3)` | Sends `https` requests over QUIC. |
| `with_interface` | Binds every connection to a network interface. |
| `with_resolver` | Substitutes for `net.lookup` on host names; TLS keeps the host name for SNI and certificate checks. |
| `with_cancel` | Makes a request fail with `Io(Aborted)` once its token is cancelled. |

### HTTP URLs

URLs: `parse_url`, `resolve` and `with_query_pair` build on `std/uri`.

### HTTP connections

`connect` returns a droppable `Connection` that closes its stream when released. `detach` hands the stream on.

### HTTP proxies

Proxies: `with_proxy`, `with_http_proxy`, `with_https_proxy`, `with_no_proxy`, `with_proxy_from_environment` and `with_dialer` send requests through an `http://` proxy (absolute-form for `http` targets, `CONNECT` for `https` targets) or a `socks5://`, `socks5h://`, `socks4://` or `socks4a://` proxy.

`no_proxy` follows Go's matching rules.

### HTTP content coding

The one content-coding table gives each coding its encoder and decoder. `accepts_coding`, `negotiate_coding` and `encode_body` read it. The client's decompression and `std/web` compression and static files share it.

### HTTP cookies

`Cookie` is the one cookie model. The jar stores it, `request_cookies` reads the `cookie` headers into it, and `set_cookie` and `format_set_cookie` write it. `std/web` sessions use it.

### HTTP cookie jar

`CookieJar` is a shared RFC 6265 jar (`cookie_jar`, `cookie_jar_on` for a jar that reads a `clock.Clock`, `with_cookie_jar`, `jar_set`, `jar_header`, `jar_cookies`, `jar_clear`).

- It enforces the `Secure` rules and the `__Secure-` and `__Host-` prefixes.
- It rejects a `Domain` that is a public suffix other than the request host.
- It converts a `Domain` and a URL host with non-ASCII characters to their `xn--` form.

### HTTP streamed responses

A `Response` whose `stream` is a `chan[u8[]]` is written as it is produced, chunked on HTTP/1.1 and as DATA frames on HTTP/2, until the handler closes the channel. Its `finish` function, when set, returns the trailer fields sent after the last chunk.

### HTTP duplex serving

Duplex serving: `serve_duplex_conn_on` and `serve_duplex_tls_conn_on` serve one HTTP/2 connection whose handlers receive `Request.upload` while they answer. They stop accepting streams with a `GOAWAY` when the drain channel closes. `receive_body` and `receive_body_until` read an `Upload`.

### HTTP calls

Calls: `open_call`, `call_send`, `call_receive` and `call_close` run one HTTP/2 exchange over the client's connection with a request body that is sent as the caller produces it and a response read as `CallEvent`s.

## std/http/websocket

Operations: `receive`, `send_text`, `send_binary`, `ping` and `close`, with `_on` forms, and `adopt` and `detach` on the `*SocketHandle`.

## std/http/sse

Shared: `Event` (`id`, `event`, `data`, `retry`), `format`, `comment` and `keepalive`. Server: `stream`, `stream_with`, `serve_events`, `serve_events_with`, `Emitter` and `last_event_id`. Client: `parser`, `feed`, `parse` and `listen`, which reads a `text/event-stream` response with any chunking and CR, LF or CRLF line ends.

## std/http/files

Content type comes from the extension, then sniffing. Weak `ETag` and `Last-Modified` work with `If-None-Match` and `If-Modified-Since`. A single `Range` is answered as 206 or 416, with `If-Range`. `HEAD` is supported and the body is streamed.

## std/http/huffman

`decode` fails `Malformed` on the EOS symbol and on padding that is longer than 7 bits or not all ones, and never panics on any input.

## std/http/qpack

`Decoder` (`decoder` takes the maximum table capacity, the blocked stream limit and the field section size limit) keeps the dynamic table:

| Function | What it does |
| --- | --- |
| `on_encoder_stream` | Consumes encoder stream bytes split anywhere, buffers at most 64 KiB of an incomplete instruction and returns the blocked streams that can now decode. |
| `decode` | Returns `Decoded`, either `Fields` or `Blocked`, and queues a Section Acknowledgment. |
| `cancel_stream` | Queues a Stream Cancellation. |
| `take_decoder_stream` | Returns the queued decoder stream bytes plus one Insert Count Increment. |

`Encoder` writes field sections with the static table only, so they never block and nothing goes on the encoder stream. `on_decoder_stream` accepts Stream Cancellations and fails `Malformed` on any other instruction.

`decompression_failed`, `encoder_stream_error` and `decoder_stream_error` return the HTTP/3 codes `0x200`, `0x201` and `0x202` for failures of `decode`, `on_encoder_stream` and `on_decoder_stream`.

Every entry point fails `Malformed` on hostile input: integers are capped at 2^62-1, and strings, indexes, Required Insert Counts, Bases, entry sizes and capacities are checked before use.

`static_table`, `insert_count`, `table_size` and `table_entries` expose the tables.

## std/web

`static` and `static_with` serve a directory. `events` and `event_stream` serve server-sent events.

## std/tls

`client`, `server` and `dial` return a droppable `Session` that sends `close_notify` and closes the stream beneath it when released. Its `handle` is the shared `*SessionHandle` holding `protocol`, `version`, `group`, `suite`, `cipher_suite` and `peer_certificates`. `client_handle` and `server_handle` return the view alone, and `read_on`, `write_on`, `close_on` and `stream_on` operate on it.

`quic_client` and `quic_server` run the TLS 1.3 handshake of RFC 9001 and return a `QuicResult`. They work through a `QuicTransport` of callbacks. The callbacks send and receive handshake bytes per `Level` and install each `QuicSecret`. `suite_hash`, `key_size`, `cipher` and `expand_label` derive keys for a `Suite`.

| Field | What it holds |
| --- | --- |
| `Config.staples` | The OCSP responses a server staples. |
| `Config.revocation` | The revocation check of a client. See Certificate status. |
| `Config.credentials` | A `CredentialSource`. Its `choose` takes a `HelloInfo` (server name and offered protocols). It returns the `Credentials` (chain, key, staples and the protocol to select) of one handshake. |

| Part | Public as |
| --- | --- |
| TLS 1.2 key schedule, public for `std/dtls` | `suite12_of`, `prf`, `extended_master_secret`, `verify_data`, `key_block12` and its `KeyBlock12`. |
| Key exchange, public for `std/dtls` too | `Share`, `ephemeral_share` and `shared_secret`. |
| TLS 1.3 key schedule, public for `std/dtls` as well, with the label prefix as a parameter | `suite13_of`, `expand_label_with`, `key_schedule` and its `KeySchedule`, `derive_secret`, `finished_mac` and `export_secret`. |

`retry_random`, `message_hash`, `verify_content` and `downgrade_sentinel` serve the handshake.

## std/serde

Readers share `parse_integer`, `parse_unsigned` and `syntax_at`. `syntax_at` builds the `Syntax` error for a line and column. The helpers the derived code calls are private.

`Tree`, `Node` and `Member` are the value shape that CBOR, MessagePack, TOML, YAML and template data share. `to_tree` builds one from any value, and `tree_reader` reads a `T` from one.

`key_text` and `unsigned_tree` hold the key text and the `u64` range rule.

A format maps its own value type `V` to a tree with `node_of` and a `Shape[V]` function, and back with `value_of` and a `Rebuild[V]`.

## std/json

A reader and writer over UTF-8 bytes, the `Value` document model with member builders (`text_member`, `int_member`, `bool_member`, `base64url_member`, `object_member`, `array_member`) and typed getters (`field`, `field_text`, `field_int`, `text_of`, `int_of`), `equal` (compares numbers numerically and objects without regard to member order), and `serialize[T]` and `deserialize[T]` with naming policies.

## std/json/pointer

Edits return a new document.

## std/json/schema

`compile` and `compile_with` check the schema, resolve `$ref`, `$anchor`, `$dynamicRef` and registered resources, and precompile patterns. `validate` returns `Violation`s with the instance pointer, the keyword pointer and a message. `unevaluatedProperties` and `unevaluatedItems` use annotations from the branches that passed. `format` annotates or asserts.

## std/sql

### SQL connection handle

`ConnectionHandle` is the copyable view of closures that `std/orm` and `std/sql/pool` use. The owner holds it in its public `handle` field, and `adopt` makes an owner from one. The rest are `Value`, `Rows`, `Error`, `Options` and `Dialect`.

### SQL streaming and cancellation

Besides `query`, `each` and `execute`, a handle has `stream`. It passes each result set of a call in order as `Event`s:

- `Columns`
- one `Row` per row
- `Done` with its count

The count is `none` when the server reported none and `some(0)` when it reported zero.

| Operation | What it does |
| --- | --- |
| `cancel` | Another green thread calls it to stop the running call with `Cancelled` and leave the connection usable. |
| `on_notice` | Sets the function that receives the connection's `Notice`s. These are each message the server sends as a `Diagnostic`, PostgreSQL notifications, and a `Setting` when the server changes a session setting. |

### SQL result sets and columns

`query_all` returns every result set of a call with its count. That count, `Rows.count` and `execute` are 0 where the server reported none.

A `Column` carries `precision`, `scale` and `length`.

- `scale` is also the fractional-second digits of a time type.
- `length` is -1 when unbounded.

Each of the three is 0 when the server states none.

A `Column` carries `table` and `number`, the OID of the table the column comes from and its attribute number there, which only PostgreSQL reports.

### SQL describe and prepared statements

| Function | What it does |
| --- | --- |
| `describe` | Parses a statement and returns its columns and parameter type names as a `Description` without running it. |
| `stream_prepared` | Runs exactly one statement as a prepared statement and reports events as `stream` does. Text holding a second statement therefore fails, also without parameters. |

Both fail with `Unsupported` on a handle that does not set them, which every driver but PostgreSQL leaves as it is.

### SQL server text and errors

| Name | What it does |
| --- | --- |
| `Options.server_text` | Asks PostgreSQL and SQL Server for the server's exact text, as `Text`, in place of decoded values. |
| `Error.Server` | Holds every error of the call in order as `Diagnostic`s, each with a `Level`, a code, a message and the fields its protocol carries. |

`server_error` builds one from a code and a message.

### SQL dialects

A `Dialect` is a record. It owns everything that differs between databases:

- the OpenTelemetry system name
- default port
- placeholder prefix
- identifier quotes
- generated-key and paging syntax
- column types
- the conditional `CREATE TABLE`
- the session lock statements
- the write-transaction statement

`postgres()`, `sqlserver()`, `sqlite()`, `mysql()` and `mariadb()` build the five. The `numbered_placeholders` flag, the `LastInsertId` key return, the `LimitUnbounded` paging and the `Upsert` clause cover what MySQL does differently. `upsert_clause` writes `ON CONFLICT` or `ON DUPLICATE KEY UPDATE`. `std/orm` and `std/sql/migrate` read the one a connection carries.

### SQL transaction rollback

`rolled_back` rolls a transaction back after an error and reports the rollback's own error when it fails.

## std/sql/driver

A `Session` holds the busy, cancelled and closed state and the notice function. Every operation of a connection runs as a call on the session, which fails with `Busy` or `Closed` and abandons the connection on an error the driver calls fatal. `establish` runs the login under it.

| Function | What it does |
| --- | --- |
| `collect` | Buffers a call's events into its last result set. |
| `rows` | Turns an `each` visitor into an event callback. |
| `ignore_event` | Discards events. |
| `report`, `setting`, `listener` | `report` passes a diagnostic and `setting` a session setting to the notice function, and `listener` makes the `on_notice` that replaces it. |
| `dial` | Opens the socket at the dialect's default port. |
| `traced` | Wraps a handle with `otel.instrument_sql`. |
| `call` | Runs an operation that is not part of the handle, such as a ping, as a call on the session. |
| `connection` | Guards, traces and adopts a driver's handle, and passes `cancel` to the driver only while a call runs and once per call, so `std/sql/postgres`, `std/sql/sqlserver`, `std/sql/mysql` and `std/sql/sqlite` do not import `std/otel`. |

## std/sql/mysql

`connect`, `connect_tls`, `connect_with`, `connect_stream`, a `State` that tracks the server, thread id, schema and transaction, `begin_with` for an isolation level, and `savepoint`, `rollback_to` and `release`.

## std/sql/sqlite

| Function | What it does |
| --- | --- |
| `open`, `open_with` | (read-only, create, busy timeout) |
| `exec` | Runs scripts. |
| `query`, `each`, `stream`, `execute` | Take `?` or `?N` parameters over cached prepared statements. |
| `cancel` | Interrupts the running statements through `sqlite3_interrupt` from any green thread, and each fails with `sql.Error.Cancelled`. |
| `close`, `close_on` | Are idempotent. |
| `connection` | Presents a database as an `sql.Connection`. The one `connection` returns leaves closing to the `Database`. |
| `connect` | Presents a database as an `sql.Connection`. The one `connect` returns closes it. |

Also `last_insert_id`, `begin`, `begin_immediate`, `commit` and `rollback`, and `backup` to a file through the online backup API.

A closed `Database` fails with `sql.Error.Closed`. Calls block their OS thread.

## std/sql/pool

`open` takes a `Config` (`max`, `max_idle`, `idle_timeout`, `max_lifetime`, `wait_timeout`, `ping_after`, `ping`, `clock`). `acquire` and `acquire_until` (with a cancel token) park in first-in first-out order while every connection is in use. A droppable `Lease` returns its connection on release, and discards the notice function set through it. A call that fails with `Cancelled` leaves the connection in the pool. Also `with`, `discard`, `prune`, `stats` and `shutdown`.

## std/sql/migrate

`from_text` and `load_dir` read `-- +up` and `-- +down` files named `NNN_name.sql`.

## std/dbus

The `Value` enum and `Type` signatures with `encode`, `decode`, `encode_message` and `decode_message`; addresses, SASL authentication and the session and system buses; method calls, signals, match rules, name ownership, properties and introspection; exported objects. See `### D-Bus`.

## std/portal

See `### Desktop portals`.

## std/atspi

See `### Accessibility`.

## std/compress/bzip2

The decoder reads multiple streams and blocks and checks both CRCs. Randomised blocks fail with `Unsupported`.

## std/compress/codec

A growable `Buffer` that packs bits, a `BitReader`, two-level Huffman decode `Tables`, a length-limited Huffman `Builder`, hash-chain `Chain` and `walk`, a shared LZ77 `History` window (`history`, `append`, `index_to`, `extend_back`), the one-shot `decompress` scaffold with `output_for`, `deliver` and `each_chunk`, the streaming `Encoder` shell (`encoder`, `encode`, `flush`, `finish`), `copy_match`, and LEB128 varints.

## std/hash

Also FNV-1 and FNV-1a in 32 and 64 bits; SipHash-2-4 and SipHash-1-3 with a 16-byte key in 64- and 128-bit forms; MurmurHash3 (`murmur3_32`, `murmur3_128`); wyhash final 4.2; `jump` consistent hashing; and `rendezvous` hashing.

## std/bits

`std/hash`, `std/rand`, `std/text`, the JSON, XML and YAML readers and the compressors use these for rotations, bit counts and bit lengths.

## std/crypto

The source is `getrandom` on Linux, `ProcessPrng` on Windows, and the host's `crypto.getRandomValues` through `low.entropy` for `browser/wasm32`. `wipe` zeroes a byte slice. A failing operating system source panics.

A refill runs 12 ChaCha20 blocks under the key with `aead.chacha20_blocks`. It replaces the key with the first 32 bytes of the first block (fast key erasure) and keeps the other 736 bytes as a buffer. A request copies its bytes out of the buffer and overwrites them with zeros. So memory after a request holds only keystream that no request has taken. It holds neither the key that produced its bytes nor the bytes handed out.

The rounds load and store every working word in the generator's own block, so the working state is never written to the stack. The block's working words are zeroed when the refill ends.

The key is replaced from the operating system after 1 MiB of output and in a forked child.
| Platform | Fork check |
| --- | --- |
| Linux | A child is recognised by a page mapped with `MADV_WIPEONFORK` (`sys.map_fork_marker`). The page's first word is 1 in the parent and 0 in the child, so a request pays one load. |
| Linux, when the page cannot be mapped | The request compares the process id. |
| Windows and `browser/wasm32` | Have no `fork` and check nothing. |

The generator state is the module's 1024-byte `__static_block`. Every green thread and OS thread shares it, and a `__lock` word in it guards it. It is not kept per OS thread, because a green thread moves between OS threads, so per-thread state would give no exclusion on its own.

A holder of a `__lock` is never preempted, and the collector does not run until it returns. So the lock is held only across loops over the block, never across an allocation or a system call. The result is allocated, the operating system source is read and the marker page is mapped before or after it.

Requests over 4096 bytes are served in 4096-byte chunks, each taking the lock once.

## std/crypto/limb

`carry` and `borrow` out of a 64-bit step, `add`, `sub` and `add_masked` over limb slices, `from_bytes` and `to_bytes` between big-endian bytes and limbs, and the constant-time `same`, `mask_of`, `blend`, `choose`, `gather`, `equal` and `is_zero`. `same_limbs` and `zero_limbs` give the comparison of whole slices as `1` or `0` for a mask. `std/crypto/nistec`, `std/crypto/rsa`, `std/crypto/edwards25519` and `std/crypto/pairing` build on it.

## std/crypto/drbg

`instantiate` (entropy, nonce, personalization) with the derivation function, `instantiate_without_df` (exactly 48 bytes of entropy, personalization of at most 48) without it, `reseed`, `generate` with additional input and `uninstantiate`, on a `*CtrDrbg`.

Without the derivation function, reseed entropy is exactly 48 bytes and additional input at most 48, padded with zeros. This is the generator the NIST post-quantum known-answer files are drawn from.

A request is at most 65536 bytes (2^19 bits). `generate` fails with `ReseedRequired` after 2^48 requests.

The caller supplies all entropy; the module reads no entropy source.

Every operation zeroes the copies of the entropy and additional input, the derived seeds, the intermediate keys and the AES key schedules it makes. So a `*CtrDrbg` holds the only secrets the generator keeps.

The AES block function is `std/crypto/aead`'s.

## std/crypto/aead

The AES block functions for 128-, 192- and 256-bit keys (`aes_key`, `encrypt_block_into`, `decrypt_block`, `decrypt_block_into`) and `aes_wipe`, which zeroes a key schedule. The ChaCha20 functions `chacha20_block` (the RFC 8439 block function), `chacha20_state` and `chacha20_blocks` (the block function over a 32-word state the caller owns, whose rounds load and store every working word in that state and zero it afterwards, so no word of it is held in a spilled register; `std/rand`'s `chacha` and `crypto.random` build on them), `chacha20_xor` (takes the RFC 8439 block counter, on the same rounds) and `hchacha20`.

## std/crypto/md5

For interop and checksums, not security.

## std/crypto/cipher

There is no ECB, and CBC provides no integrity: use an `Aead`.

## std/crypto/edwards25519

The extended `Point` with `base`, `identity`, `add`, `negate`, `double`, `multiples` and the constant-time windowed `multiply`, the RFC 8032 point `encode` and `decode`, and the scalars modulo the group order `L` (`reduce`, `mul_add`, `canonical`). `std/crypto/ed25519` and `std/crypto/ristretto255` build on it.

## std/crypto/ristretto255

`Element` with `identity`, `generator`, `decode` (canonical encodings only, `none` for any other), `encode`, `equal`, `add`, `sub`, `neg`, the constant-time `multiply` and `multiply_base`, and `from_uniform`, the one-way map of 64 bytes. Scalars are 32 little-endian bytes below `L`: `scalar_from_bytes`, `scalar_from_wide`, `scalar_add`, `scalar_sub`, `scalar_mul`, `scalar_neg`, `scalar_invert`, `scalar_is_zero` and `scalar_random`.

## std/crypto/ecdsa

It owns the curve sizes (`scalar_size`), the named-curve lookup (`curve_by_oid`) and the uncompressed point encoding (`uncompressed`, `coordinates`) that `std/crypto/x509`, `std/crypto/jose` and `std/webauthn` use.

## std/crypto/nistec

`Field` is Montgomery arithmetic modulo an odd prime of 4 or 6 limbs (`add`, `sub`, `mul`, `to_mont`, `from_mont`, `power`, `invert`, `reduce_wide` and `reduce_mont`).

| Function | What it does |
| --- | --- |
| `reduce_mont` | Reduces big-endian bytes of any length in a time that depends only on the length. |
| `mul` | Runs unrolled for 4 limbs. |
| `power` | Steps through 4-bit windows of its public exponent. |

`std/crypto/pairing` builds its fields on `Field`.

`Group` holds the curve (`p256`, `p384`) and has the complete projective `point_add`, `negate`, the constant-time `scalar_mul`, `affine`, the uncompressed and compressed SEC1 encodings (`encode_point`, `compress`, `decompress`) and `point_at` with its on-curve check.

The scalars modulo the group order are big-endian bytes with `order_from_bytes`, `order_reduce`, `order_add`, `order_sub`, `order_mul`, `order_neg`, `order_invert` and `order_random`.

## std/crypto/hashtocurve

The forms RFC 9497 uses are `hash_to_scalar_p256`, `hash_to_ristretto255` (appendix B of RFC 9380) and `hash_to_ristretto255_scalar`.

## std/crypto/pairing

`bn254` (alt_bn128, the Ethereum curve) and `bls12_381` return a `Curve` with the base and scalar fields as `nistec.Field`s, the generators `g1` and `g2` and the constants of the twist. Points of `G1` over `Fp` and of `G2` over `Fp2` are `Point`s in Jacobian coordinates, with the complete `add`, `double`, `neg`, `equal`, `affine`, `from_affine`, `on_curve` and `in_subgroup`. `field_add` to `field_power` work in either coordinate field.

| Function | What it does |
| --- | --- |
| `mul` | A 4-bit window that scans the whole table, for secret scalars. |
| `mul_public` | Double-and-add. |
| `msm` | Pippenger's bucket method for public scalars. |
| `msm_secret` | Reads and writes every bucket for secret ones. |

`pair` is the optimal ate pairing. It runs a Miller loop, then the final exponentiation by `(p^12 - 1)/r`.

- For BN the loop runs over the NAF of `6x + 2` with the two Frobenius lines.
- For BLS12-381 the loop runs over the bits of `abs(x)` with a conjugation for the negative `x`, and the exponentiation is by three times that for BLS12-381, which gives the values gnark-crypto gives.

| Function | What it does |
| --- | --- |
| `check` | Multiplies several pairings with one final exponentiation. |
| `gt_one`, `gt_mul`, `gt_pow`, `gt_equal` | Work in `GT`. |

| Functions | Running time |
| --- | --- |
| `mul`, `msm_secret`, `add`, `double` and the field functions | Run in a time that does not depend on the values, except for the exponent of `field_power`. |
| `pair`, `check`, `mul_public`, `msm`, `in_subgroup` and `gt_pow` | Take their inputs as public and do not. |

## std/crypto/pasta

`fp` and `fq` return the two prime fields as `nistec.Field`s.

The `field_*` functions (`add`, `sub`, `mul`, `square`, `neg`, `pow`, `invert`, `sqrt`, `is_square`, `from_u64`, `from_bytes`, `to_bytes`, `from_uniform` for 64 little-endian bytes reduced by the modulus, `root_of_unity`, `delta` and the rest) work on Montgomery-form limbs. `from_bytes` accepts only the canonical 32-byte little-endian form.

`pallas` and `vesta` return a `Curve` with its `base` and `scalar` fields and `zeta`, a primitive cube root of unity of the scalar field. A `Point` is Jacobian.

The group operations are `identity`, `generator`, `double`, `add`, `neg`, `sub`, `equal`, `mul`, `msm`, `normalize`, `coordinates`, `to_bytes` and `from_bytes`:

| Operation | What it does |
| --- | --- |
| `generator` | `(-1, 2)`. |
| `msm` | Interleaved width-w non-adjacent forms over tables of odd multiples for few points and bucket accumulation over signed digits for many, chosen by an estimate of their cost, with zero scalars skipped and the points in affine form. |
| `normalize` | Points to `z = 1` with one inversion. |
| `to_bytes`, `from_bytes` | 32 bytes: `x` and the parity of `y` in the top bit, all zero for infinity. |

`hash_to_curve(c, prefix, message)` is the `pasta_curves` hash to curve: `expand_message_xmd` over Blake2b-512, the simplified SWU map on the isogenous curve and the 3-isogeny.

The group operations and `field_sqrt` are not constant time: they branch on points and on scalar bits.

## std/crypto/bn254

They pad short input with zeros and ignore the bytes after the operands.

| Function | Format |
| --- | --- |
| `decode_g1` and `encode_g1` | Read and write a `G1` point as 64 bytes, `x` then `y`. |
| `decode_g2` and `encode_g2` | Read and write a `G2` point as 128 bytes with each `Fp2` element written imaginary part first. |

All zeros is the point at infinity.

| Fails in | Error | For |
| --- | --- | --- |
| Decoding | `Coordinate` | An integer not below `p`. |
| Decoding | `NotOnCurve` | A point off the curve. |
| Decoding | `NotInSubgroup` | A `G2` point outside the subgroup of order `r`. |
| `ec_pairing` | `Length` | Input that is not a whole number of 192-byte pairs. |

`ec_mul` multiplies in constant time. Decoding and the pairing take public data.

## std/crypto/bls12381

`decode_g1` and `decode_g2` read the zkcrypto format of the IETF pairing-friendly curves draft, compressed (48 and 96 bytes) or not (96 and 192). The compression, infinity and sort flags sit in the top three bits of the first byte, and each `Fp2` element is written `c1` then `c0`.

| Function | What it does |
| --- | --- |
| `decode_g1` and `decode_g2` | Fail with `Length`, `Flags`, `Coordinate`, `NotOnCurve` or `NotInSubgroup`. They run in a time that depends on the point. |
| `compress_g1`, `serialize_g1`, `compress_g2` and `serialize_g2` | Write the format. |
| `hasher` | Prepares the RFC 9380 suites `BLS12381G1_XMD:SHA-256_SSWU_RO_`, `BLS12381G1_XMD:SHA-256_SSWU_NU_` and their `G2` forms. |
| `hash_to_g1`, `encode_to_g1`, `hash_to_g2` and `encode_to_g2` | Run them with the simplified SWU map, the 11-isogeny to `E` and the 3-isogeny to `E'` and cofactor clearing by `h_eff`. They run in a time that depends on the length of the message and not its bytes. |

## std/zk/groth16

A circuit is a rank-1 constraint system `<a, z> <b, z> = <c, z>` over an assignment `z`. The assignment holds the constant one, the public inputs and then the private values.

| Function | What it does |
| --- | --- |
| `Builder` (`builder`, `input`, `secret`, `term`, `scaled`, `constrain`, `product`, `system`, `assignment`, `inputs_of`) | Records a circuit. |
| `read_r1cs` | Reads a circuit from a circom `.r1cs`. |
| `read_witness` | Reads its `.wtns` witness. |
| `write_r1cs` and `write_witness` | Write the same files. |
| `setup` | Makes a `ProvingKey` and its `VerifyingKey` from secrets drawn from `crypto.random` and wiped before it returns. |
| `prove` | Makes a `Proof` from the key, the system and an assignment. |
| `verify` | First checks that each proof point is on its curve and in the subgroup of order `r`. It checks the proof against the public inputs with one product of four pairings. |
| `proof_to_json`, `proof_from_json`, `verifying_key_to_json`, `verifying_key_from_json`, `public_to_json` and `public_from_json` | Are the snarkjs `proof.json`, `verification_key.json` and `public.json`. |

The setup is a single party. Whoever runs it and keeps the secrets can forge proofs. So keys from `setup` suit tests and settings where the prover is the verifier. A snarkjs `.zkey` is not read.

`prove` and `setup` meet the private values, the blinding scalars and the setup secrets only in constant-time field arithmetic, in transforms whose access pattern follows the size of the system, and in `pairing.mul` and `pairing.msm_secret`, so their running time follows the system, not those values. `verify` and the JSON and file readers handle public data and run in a time that depends on it.

## std/zk/goldilocks

| Functions | Work on |
| --- | --- |
| `add`, `sub`, `neg`, `mul`, `pow`, `inv`, `batch_inv` and `evaluate` | `u64` values below `modulus` |
| `ext`, `ext_add`, `ext_sub`, `ext_neg`, `ext_mul`, `ext_scale`, `ext_inv`, `ext_pow`, `ext_batch_inv`, `ext_evaluate` and `ext_evaluate_base` | `Ext` |

| Function | What it does |
| --- | --- |
| `root_of_unity(log)` | Is the root of order `2^log` that Winterfell's `get_root_of_unity` returns. |
| `twiddles(size)` | Precomputes a radix-2 transform. |
| `ntt` and `intt` | Evaluate at the powers of that root and interpolate. |
| `coset_ntt` and `coset_intt` | Do the same on `shift * w^i`. |
| `ext_ntt`, `ext_intt`, `ext_coset_ntt` and `ext_coset_intt` | Transform each coordinate of extension values. |

The arithmetic branches on its operands, so none of it is constant time.

## std/zk/stark

### STARK computations

A computation is an `Air`:

| Part | Meaning |
| --- | --- |
| `width` and `steps` | `width` columns of `steps` rows (a power of two from 4 to `2^26`) |
| `transitions` | Zero between each row and the next |
| `boundaries` | Fix cells (the public inputs, as `Boundary { column, row, value }`) |
| `periodic` | Columns that repeat with a power-of-two period |

A constraint is an `Expr` built with `constant`, `current`, `next`, `periodic`, `add`, `sub`, `mul` and `pow`. `degree` reads its degree from the tree, so no degree is declared and none can be understated.

### STARK parameters

`Params` fix `blowup`, `queries`, `grinding_bits` and `zero_knowledge`. A verifier fixes them itself and never takes them from the prover.

| Parameter | Range |
| --- | --- |
| `blowup` | 2 to 64, at least the number of composition columns the constraints need |
| `queries` | 1 to 1024 |
| `grinding_bits` | 0 to 32 |

### STARK proving and verifying

| Function | What it does |
| --- | --- |
| `prove` | Makes a `Proof` from an `Air`, `Params` and a column-major trace. It fails with `Params`, `Air`, `Trace`, `Unsatisfied` or `TooLarge`. |
| `verify` | Returns whether the proof holds and never panics on a malformed one. |

### STARK proof construction

The proof:

1. Commits the trace on the coset `7 * <w>` of size `steps * blowup`, with Blake3 Merkle trees whose leaf `j` holds the rows at the points `x` and `-x`.
2. Commits the composition polynomial split into columns of degree below `steps`.
3. Evaluates both at an out-of-domain point `z` and `z` times the generator of the trace domain.
4. Proves with FRI that the DEEP quotient has low degree.

FRI folds by 2, commits layers from the second, and sends the last polynomial of at most 16 coefficients in the clear.

Queries and the combination coefficients come from a Blake3 Fiat-Shamir transcript that starts from the parameters and the exact encoding of the `Air`. So a proof does not verify under another statement. The grinding nonce is found after every commitment. Each query opens the trace, the composition and every FRI layer, and the openings of one tree share their sibling hashes.

### STARK proof encoding

`write_proof` and `read_proof` turn a `Proof` into bytes. The bytes are, in order:

1. A magic
2. A header naming the parameters, the width and the number of steps
3. The roots
4. The out-of-domain values
5. The remainder
6. The nonce
7. The openings

Each number is 8 little-endian bytes below the modulus.

| `read_proof` error | When |
| --- | --- |
| `Malformed` | A proof cut short, with bytes left over or with a number at or above the modulus. |
| `Mismatch` | The header names other parameters or another shape. |

### STARK security levels

| Function | Returns |
| --- | --- |
| `security` | The level in bits as `Security`. |
| `conjectured` | `queries * log2(blowup)` plus the grinding bits once the queries alone give 80 bits, at most 191 and at most 128 for the 256-bit hashes. |
| `proven_unique` and `proven_list` | The levels of the proximity gap theorems of eprint 2024/1553 in the unique and the list decoding regimes, computed as Winterfell does. |

The proven levels are what holds without the proximity gap conjecture. The proven levels are far below the conjectured one for small parameters, so choose the `Params` from `security`, not from the conjecture alone (see `### STARK security`).

### STARK zero knowledge

With `zero_knowledge` the trace is padded to `steps * 2^s` rows, the smallest with at least `4 * queries + 2` rows added. The added rows are random values from `crypto.random`, so each trace column takes uniformly random values at the `2 * queries + 2` points that a proof reveals. With `zero_knowledge` an extra random polynomial is added to the DEEP polynomial that FRI sees.

That is all the hiding there is. The openings of the composition columns are not masked, the leaves are not salted, no simulator argument is made, and `prove` runs in a time that depends on the trace. Without `zero_knowledge` the proof is succinct and not hiding.

## std/zk/airs

`fibonacci` and `fibonacci_trace` (two columns, degree 1, the last value public) and `hash_chain` and `hash_chain_trace` (a Feistel chain `(l, r) -> (r + (l + k)^3, l)` with a periodic round constant column, degree 3, the output public and the input private), with `round_constants` giving the 16 constants.

## std/zk/halo2

So a circuit lives in the scalar field of Vesta, the base field of Pallas, and its commitments are Vesta points (`std/crypto/pasta`). The proof encoding, transcript and verifying key hash are those of `halo2_proofs`. So a proof made here verifies there and the other way round.

### Halo2 parameters

| Function | What it does |
| --- | --- |
| `params(c, k)` | Derives the `2^k` generators, their Lagrange form and the points `w` and `u` by hash to curve of the domain `Halo2-Parameters`, so nobody holds a secret. |
| `write_params` and `read_params` | Use the `halo2_proofs` file layout. |
| `commit` and `commit_lagrange` | Are Pedersen vector commitments with a blinding point. |

### Halo2 constraint systems

A circuit is a `ConstraintSystem` made by `constraint_system`, with `advice_column`, `fixed_column` and `instance_column`, and with `selector` and `complex_selector`.

`query_advice`, `query_fixed` and `query_instance` read a cell at a row offset, and `query_selector` reads a selector. Each returns an `Expr` that `constant`, `neg`, `add`, `sub`, `mul` and `scale` combine.

| Function | What it does |
| --- | --- |
| `create_gate` | Adds expressions that must vanish on every usable row. |
| `lookup` | Adds an input and a table of expressions. |
| `enable_equality` | Lets a column take part in equality constraints. |
| `enable_constant` | Makes a fixed column, with equality enabled, hold the constants the regions ask for. |

### Halo2 selector rules

| Selector | Where it may appear |
| --- | --- |
| Simple selector | It may only multiply a gate. It may never take part in a sum, beside another simple selector in a product, or in a lookup. |
| Complex selector | It may appear anywhere. |

### Halo2 blinding factors

`degree`, `blinding_factors` and `minimum_rows` report the shape. The blinding factors are the largest number of queries of one advice column, at least 3, plus 2. The last `blinding_factors + 1` rows of every column are unusable.

### Halo2 assignments and layouter

An `Assignment` holds what a circuit assigns: the values of each fixed and advice column, the rows where each selector is on, and the `Equal` constraints between `Cell`s in the order they are made. A `Layouter` builds one as the `SimpleFloorPlanner` of `halo2_proofs` does.

| Function | What it does |
| --- | --- |
| `layouter(params, cs, instances)` | Starts it. |
| `region` | Opens a region whose cells sit at row offsets (`assign_advice`, `assign_fixed`, `enable_selector`, `copy_advice`, `assign_advice_from_constant`, `assign_advice_from_instance`, `constrain_equal` and `constrain_constant`, each assigned cell an `AssignedCell` with its value). |
| `end_region` | Places the region at the first row at which none of its columns and selectors is taken. It then puts the region's constants in the next free rows of the first constants column, each constrained equal to its cell. |
| `constrain_instance` | Ties a cell to a public input. |
| `assign_table` | Loads a lookup table into a fixed column and fills the remaining usable rows with its first value. |
| `finish` | Returns the `Assignment`. |

Regions and cells carry no names, since names in `halo2_proofs` reach neither the key nor the proof.

### Halo2 key generation

`keygen(params, cs, assignment)` reads the fixed values, the selectors and the equalities. It combines the selectors into fixed columns after the fixed columns of `cs` as `halo2_proofs` 0.3.0 does.

| Selectors | Column |
| --- | --- |
| A complex selector, or one in no gate | Takes a column of its own. |
| Simple selectors that are never on in the same row | Share a column, the `k`th of them on the rows where it holds `k`, while their gates stay within the degree of `cs`. |

It returns a `ProvingKey` with its `VerifyingKey`, leaving `cs` as it was. The key follows from those inputs alone, so the prover and the verifier derive it independently, the verifier from a layout with any witness. `pinned` returns its `halo2_proofs` description.

### Halo2 proving

`prove(params, pk, circuits, instances, random)` makes one proof for several circuits, as `create_proof` does. Each `Assignment` of `circuits` gives its advice columns, and the entry of `instances` at the same position gives its instance columns. It takes a `random` function (use `crypto.random`). It runs the lookup permutation, the permutation argument, the vanishing argument, the multi-point opening and the inner product argument. It fails with `Instance`, `Circuit` or `Unsatisfied` when a gate, an equality or a lookup does not hold.

### Halo2 verification

`verify(params, vk, instances, proof)` checks a proof of as many circuits as `instances` has entries with one multi-scalar multiplication.

| `verify` error | When |
| --- | --- |
| `Instance` | The instance columns do not fit. |
| `Malformed` | The proof is cut short, has bytes left over, or holds a non-canonical scalar or a point that is not on the curve or is at infinity. |
| `Rejected` | The proof is well formed and wrong. |

It never panics on a malformed proof. It reads the whole proof, which `halo2_proofs` does not.

### Halo2 omissions and security

`halo2_proofs` 0.3.0 has no challenges or phases in its public API, so there are none here, and the `V1` floor planner is not provided. See `### Halo2 security` for what is and is not protected.

## std/crypto/voprf

`derive_key_pair`, `generate_key_pair` and `public_key` make keys. A `Client` calls `blind` and `finalize`, and a `Server` calls `blind_evaluate` and `evaluate`. Each batch of elements shares one DLEQ proof, which `finalize` verifies. `blind_with` and `blind_evaluate_with` take the blind and the proof scalar, for known-answer tests. Elements and scalars are serialized as the suites define, and `Error` names the RFC failures (`InvalidInput`, `Inverse`, `Verify`, `Deserialize`, `DeriveKeyPair`).

## std/crypto/rsa

`integer` strips the leading zeros that key integers from JWK and COSE carry. `mod_pow` is constant-time modular exponentiation over big-endian bytes for odd moduli of up to 8192 bits, which the SSH Diffie-Hellman groups use.

## std/crypto/blake2b

`sum` and an incremental `Hasher` (`hasher`, `update`, `finalize`, `clone`) with `Options` for the digest size from 1 to 64 bytes, a key of at most 64 bytes, a salt and a personalisation of at most 16 bytes each, and `personalised` for a 64-byte digest with a personalisation. `finalize` returns the digest of what was absorbed and leaves the hasher able to absorb more, which the Halo2 transcript relies on. `hasher` and `sum` fail with `Error` (`Size`, `Key` or `Parameter`).

## std/crypto/shamir

The coefficients of `split` come from `std/crypto`.

| Function | What it does |
| --- | --- |
| `split` | Fails with `BadThreshold`, `BadShareCount`, `EmptySecret` or `BadRandomness`. |
| `split_with` | Takes the `(k - 1) * secret.len` coefficient bytes as an argument. Fails with `BadThreshold`, `BadShareCount`, `EmptySecret` or `BadRandomness`. |
| `combine` | Interpolates at zero and fails with `TooFewShares`, `ZeroX`, `LengthMismatch` or `DuplicateX`. |

Field multiplication is a fixed eight-step shift and mask loop, and inversion is the fixed power `a^254`. So no table is indexed by, and no branch depends on, a secret byte, a coefficient or a share value. Only the x coordinates, which are public, are checked and compared.

Shares carry no check value, so fewer than `k` shares or a corrupted share decode to bytes unrelated to the secret without an error.

## std/crypto/hpke

KEMs: DHKEM(X25519, HKDF-SHA256) (`Kem.DhkemX25519`, id 0x0020), which supports the base, PSK, auth and auth-PSK modes, and MLKEM768-X25519 (`Kem.Mlkem768X25519`, id 0x647a), as `draft-ietf-hpke-pq-05` defines it over the X-Wing construction of `draft-irtf-cfrg-concrete-hybrid-kems-03` (ML-KEM-768 and X25519 combined with SHA3-256, a 32-byte private seed expanded with SHAKE256, 1216-byte public keys and 1120-byte encapsulated keys), whose `DeriveKeyPair` is the `LabeledDerive` of `draft-ietf-hpke-hpke-03`.

| KEM | Modes | An all-zero X25519 output |
| --- | --- | --- |
| `Kem.DhkemX25519` | Base, PSK, auth and auth-PSK. | Rejected with `Error.Encap` or `Error.Decap`. |
| `Kem.Mlkem768X25519` (the hybrid) | Base and PSK; it fails the authenticated ones with `Error.Unsupported`. | Takes part in the combiner as zero bytes, as the draft's group definition has it. |

| Function | What it does |
| --- | --- |
| `Suite` | Pairs a `Kem` with an `AeadId`. |
| `derive_key_pair`, `generate_key_pair`, `serialize_public_key` and `deserialize_public_key` | Handle keys as `KeyPair` byte strings. |
| `Credentials` (`base`, `psk`, `auth`, `auth_psk`) | Selects the mode from which inputs are present. |
| `setup_sender`, `setup_sender_seeded` (caller-supplied ephemeral randomness, for known-answer tests) and `setup_recipient` | Return a `*Context` with `seal`, `open` and `export`. |
| `seal_single` and `open_single` | Do one message. |

A context uses the sequence number as the nonce and fails with `Error.Limit` when it is exhausted. A failed `open` does not advance it. `seal` is for sender contexts and `open` for recipient contexts. An export-only context has only `export`.

Failures are `Error` values. Pre-shared keys are not required to be 32 bytes.

## std/crypto/prehash

The `Hash` enum of the functions FIPS 204 and FIPS 205 accept for pre-hash signing (SHA-224, SHA-256, SHA-384, SHA-512, SHA-512/224, SHA-512/256, SHA3-224, SHA3-256, SHA3-384, SHA3-512, SHAKE128 with 32 bytes of output and SHAKE256 with 64), `digest`, `oid`, the DER object identifier that HashML-DSA and HashSLH-DSA put in the signed message. `pure_message` and `prehashed_message` frame a message and context string (at most 255 bytes) as `0 || len(ctx) || ctx || message` and `1 || len(ctx) || ctx || oid || digest`.

## std/crypto/mldsa

| Function | What it does |
| --- | --- |
| `generate` and `generate_seeded` | Return `Keys`: the 32-byte seed, the public key and the expanded private key. |
| `sign`, `sign_prehash`, `sign_mu` and `sign_internal` | Take a `Randomness`. |
| `verify`, `verify_prehash`, `verify_mu` and `verify_internal` | Return `false` for every malformed key, signature, context or hint encoding. |
| `message_representative` | Computes the external `mu`. |

| `Randomness` | Value |
| --- | --- |
| `Hedged` | 32 bytes from `crypto.random` |
| `Deterministic` | 32 zero bytes |
| `Given` | A 32-byte value for known-answer tests |

The context string is at most 255 bytes.

Signing branches on no private value and indexes no table with one. The number of rejection-sampling rounds is the only thing it reveals.

Its fixtures hold NIST ACVP key generation, signature generation and signature verification vectors and vectors generated with OpenSSL.

## std/crypto/slhdsa

| Function | What it does |
| --- | --- |
| `generate` and `generate_seeded` | Return `Keys`. These hold the `3n`-byte seed (`SK.seed`, `SK.prf`, `PK.seed`), the `2n`-byte public key and the `4n`-byte private key. |
| `sign`, `sign_prehash` and `sign_internal` | Take a `Randomness`. |
| `verify`, `verify_prehash` and `verify_internal` | Return `false` for a key, signature or context of the wrong length and for every signature whose recomputed hypertree root differs. |

| `Randomness` | Value |
| --- | --- |
| `Hedged` | `n` bytes from `crypto.random` |
| `Deterministic` | `PK.seed` |
| `Given` | `n` bytes for known-answer tests |

The context string is at most 255 bytes. WOTS+, XMSS, the hypertree and FORS are module-private.

Secret seeds only enter hash inputs, so signing branches on no secret and indexes no table with one. An `s` set signs in a few seconds and key generation costs a full top-layer tree.

Its fixtures hold NIST ACVP key generation, signature generation and signature verification vectors and vectors generated with OpenSSL.

## std/crypto/falcon

FIPS 206 is not implemented, and its hashing and encodings may differ.

### Falcon verification

`verify(level, public, message, signature)` takes the public key (header byte `0x09` or `0x0a`, then `h` in 14-bit coefficients) and the signature (header byte `0x39` or `0x3a`, the 40-byte nonce and the compressed `s2`), unpadded or padded with zeros to `signature_size(level)` bytes, and returns `false` for every non-canonical encoding:

- a wrong length or header
- a public key coefficient of 12289 or more
- a compressed coefficient beyond 2047
- `-0`
- non-zero trailing bits
- partial or non-zero padding

The uncompressed signature format and the key-recovery mode are not supported.

### Falcon signing

| Function | What it does |
| --- | --- |
| `sign(level, private, message, randomness)` | Takes the private key of the known-answer files. Returns a signature padded to `signature_size(level)`. |
| `sign_internal(level, private, message, nonce, seed)` | Is the round-3 NIST API path, with the 40-byte nonce and 48-byte sampler seed the known-answer harness draws, and returns the signature at its natural length. |

The private key of the known-answer files has header byte `0x59` or `0x5a`, `f` and `g` in 6-bit or 5-bit coefficients, and `F` in 8-bit coefficients; `G` is recomputed.

When the compressed `s2` does not fit, signing samples again with the same nonce, as `falcon_sign_dyn` of the reference implementation does. `randomness` is `Hedged`, 48 bytes from `crypto.random`, or `Given` 48 bytes, from which SHAKE256 draws the nonce and then each attempt's sampler seed.

### Falcon sampling and arithmetic

Signing is the reference implementation's dynamic-tree variant. The FFT-domain LDL tree is rebuilt for each attempt, and ffSampling runs over it. SamplerZ combines the RCDT base sampler with BerExp over a ChaCha20 stream.

Signing computes in native `f64`:

| Target | `f64` is |
| --- | --- |
| amd64 | SSE2 |
| `browser/wasm32` | WebAssembly `f64` |

Both are IEEE 754 binary64 with round-to-nearest-even and no fused multiply-add, the arithmetic the reference implementation's native build relies on and the integer emulation reproduces. As in the reference implementation, the secret key reaches a branch only in the key's validity checks and each attempt's norm test, and it reaches no table index.

### Falcon key generation

| Function | What it does |
| --- | --- |
| `generate_seeded(level, seed)` | Derives `Keys` (the seed, the public key and the private key) from 48 bytes through SHAKE256, as the known-answer files do. |
| `generate(level)` | Draws the seed from `crypto.random`. |

`F` and `G` come from the reference implementation's NTRU solver. It descends through field norms with big integers held in residue number system form over 31-bit primes, and lifts back with Babai reduction in the FFT domain.

### Falcon fixtures

Its fixtures check the key pair and the full `sm` of a subset of round-3 known-answer entries, including the 1281-byte Falcon-1024 signature, and the padded signatures against `falcon_sign_dyn` of Falcon-impl-20211101.

## std/crypto/fhe/ring

The ring is `Z_Q[X] / (X^N + 1)` for a power-of-two `N` and `Q` a product of primes below 2^61 that are `1` modulo `2N`.

### FHE ring modular arithmetic

| `Modulus` holds | Functions |
| --- | --- |
| A prime with its Barrett constant `floor(2^128 / q)` | `reduce_wide`, `mul_mod`, `add_mod`, `sub_mod`, `power_mod`, `inverse_mod`, `centre` |
| Shoup's precomputed multiplication by a constant | `shoup`, `mul_shoup` |
| A 128-by-64-bit quotient | `divide_wide` |

| Function | What it does |
| --- | --- |
| `is_prime` | Is Miller-Rabin with the first twelve prime bases, which decides every 64-bit input. |
| `ntt_primes` | Lists primes of a bit size that support a degree, largest first. |

### FHE ring transforms

`Ntt` is the negacyclic number-theoretic transform of one prime (`forward`, `backward`). It uses Cooley-Tukey and Gentleman-Sande over bit-reversed powers of the smallest primitive `2N`-th root. Microsoft SEAL picks the same root, so a transform equals SEAL's.

### FHE ring bases and polynomials

A `Basis` is a list of such primes (`basis`, `slice`, `join`, `product`). A `Poly` holds one residue polynomial per prime, as coefficients or transforms by the caller's convention.

Its operations are `add`, `sub`, `negate`, `mul` of transforms, `mul_add_into`, `mul_scalar`, `to_transform`, `from_transform`, the automorphisms `X -> X^g` and `compose`:

| Operation | What it does |
| --- | --- |
| `automorphism` | The automorphism `X -> X^g` on coefficients. |
| `automorphism_table` and `permute` | The automorphism `X -> X^g` on transforms. |
| `compose` | The Chinese remainder of each coefficient as a centred `std/big` integer. |

### FHE ring basis conversions

The conversions of Halevi, Polyakov and Shoup (CT-RSA 2019) stay in the residues.

| Function | What it does |
| --- | --- |
| `extension` and `extend` | Lift coefficients to another basis exactly, choosing the representative in `-Q/2..Q/2`. |
| `scaler` and `scale` | Compute `round(t * x / D)` for `D` a leading part of the source basis. That is BFV decryption and the scaling step of the BFV product. |
| `divide_round_last` and `divide_round_last_transform` | Divide by the last prime with rounding. That is CKKS rescaling, modulus switching and the end of key switching. |

The rounding terms are summed in 128-bit fixed point rather than in floating point.

### FHE ring samplers

The samplers draw from a `Stream`:

| Function | Stream |
| --- | --- |
| `stream` | The ChaCha20 keystream of a key from `crypto.random` |
| `seeded_stream` | The ChaCha20 keystream of a given 32-byte seed |

They produce `uniform` residues, `ternary` coefficients and `gaussian` samples. For a standard deviation up to 64, `gaussian` scans a whole cumulative table cut at six deviations, so its time does not depend on the sample. Above that it uses the Box-Muller transform.

`turn` gives the cosine and sine of a rational fraction of a turn without `std/math`, so the module builds static and for `browser/wasm32`.

### FHE ring constant-time behaviour

Reduction, transforms, conversions and sampling branch on no coefficient value. `uniform` rejects samples, which reveals nothing about the accepted ones.

## std/crypto/fhe/rlwe

A `Context` holds the ciphertext primes `q_0..q_L`, one special prime `P` at least as large as each of them, and the error deviation. A ciphertext at level `l` has residues for `q_0..q_l`.

| Function | What it does |
| --- | --- |
| `chain_primes` | Picks the primes that Microsoft SEAL's `CoeffModulus::Create` picks for a list of sizes. |
| `max_modulus_bits` | Is Table 1 of the HomomorphicEncryption.org security standard for a ternary secret at 128, 192 and 256 classical bits (`Security`), as SEAL's `hestdparms.h` lists it. |
| `secret_key` | Draws a ternary secret. |
| `public_key` | Is an encryption of zero over `Q * P`. |
| `encrypt_zero` | Encrypts zero under it over `Q * P` and divides by `P` with rounding, as SEAL does, so a fresh error is the rounding error. |

`SwitchingKey` is hybrid key switching with the special prime and one digit per ciphertext prime. Digit `i` encrypts `P * g_i * s'` with `g_i` the CRT idempotent of `q_i`.

| Function | What it does |
| --- | --- |
| `switch_key` | Centres the residues of a polynomial at any level, lifts them, multiplies, sums and divides by `P`. |

Centred digits have mean zero, which keeps the switching error even across the CKKS slots. SEAL's digits lie in `0..q_i`, and their mean of `q_i / 2` gathers about 4 bits more error into the slots near `1`.

| Function | What it does |
| --- | --- |
| `relinearization_key` | Switches from `s^2`. |
| `generate` | Returns the secret, public and relinearization keys as `Keys`. |
| `galois_keys` | Switches from `s(X^g)` for the elements of a list of rotation steps and, on request, conjugation. |
| `rotation_element` | Gives `3^k` modulo `2N`. |
| `conjugation_element` | Gives `2N - 1`. |
| `relinearize` | Turns three parts into two. |
| `apply_galois` | Maps a pair through `X -> X^g` and switches it back under `s`. |

Both schemes share these steps. `phase` evaluates a ciphertext of any number of parts at the secret.

## std/crypto/fhe/bfv

| Function | What it does |
| --- | --- |
| `Parameters` | Names the degree, the plaintext modulus `t`, the ciphertext prime sizes, the special prime size and a `Security` level. |
| `context` | Rejects a product of primes above the standard's bound (`Error.ModulusTooLarge`) as well as malformed sizes. |
| `default_parameters` | Is degree 8192, `t = 65537` and primes of 43, 43, 44 and 44 bits with a 44-bit special prime: 218 bits, the 128-bit bound and SEAL's `BFVDefault(8192)`. |
| `encode` and `decode` | When `t` is a prime `1` modulo `2N`, batch `N` integers into two rows of `N/2` slots with the layout of SEAL's `BatchEncoder`, so the same values give the same plaintext. |
| `generate` | Returns the secret, public and relinearization keys. |
| `galois_keys` | Returns the keys for row rotations and the row swap. |
| `encrypt` | Adds `round(Q * m / t)` to an encryption of zero. |
| `decrypt` | Computes `round(t * phase / Q)` with the HPS scaler. |

The operations:

- `add`, `sub`, `negate`, `add_plain`, `multiply_plain`
- `multiply` (the tensor product computed by HPS: exact extension to an auxiliary basis of 61-bit primes larger than `t * N * Q`, the product there, scaling by `t / Q` into the auxiliary primes and extension back)
- `relinearize`, `multiply_relinearize`, `rotate_rows`, `swap_rows`
- `switch_down` (modulus switching to the next level)

`noise_budget` is SEAL's invariant noise budget, `log2(Q) - log2(|t * phase|) - 1`. At the default parameters a fresh ciphertext has about 150 bits of budget, and each product takes about 29. At the default parameters five successive squarings decrypt while the sixth does not, as with SEAL 4.1.

Key generation takes about 18 ms and a product with relinearization about 32 ms on one core of a 12-thread desktop.

## std/crypto/fhe/ckks

| Function | What it does |
| --- | --- |
| `Parameters` | Names the degree, the bit size of the default scale, the ciphertext prime sizes, the special prime size and a `Security` level. |
| `context` | Rejects a product of primes above the standard's bound (`Error.ModulusTooLarge`), an unlisted degree, prime sizes outside 20 to 60 bits or above the special prime, and a scale not below the first prime. |
| `default_parameters` | Is degree 8192, scale `2^40` and primes of 60, 40 and 40 bits with a 60-bit special prime, the parameters of SEAL's `ckks_basics` example. |
| `encode` and `encode_at` | Place up to `N/2` `Complex` values in the slots through the inverse canonical embedding. Slot `i` is the value at `zeta^(3^i)`, in SEAL's `CKKSEncoder` order, so the same values and scale give the same plaintext. |
| `decode` | Reverses it. |
| `encrypt` | Adds the plaintext to a public encryption of zero. |

The operations:

- `add`, `sub`, `negate`, `add_plain`, `multiply_plain`
- `multiply` (three parts, at the product of the scales)
- `relinearize`
- `rescale` (division by the last prime with rounding, dividing the scale by it)
- `multiply_rescale`
- `drop_to` (a lower level at the same scale)
- `rotate` and `conjugate`

Operands must agree in level, scale and number of parts. A product whose scale would not fit below its level's primes fails with `ScaleOutOfBounds`.

`precision` is `-log2` of the largest distance between two vectors. At the default parameters, in bits, against SEAL 4.1 at the same parameters:

| Ciphertext | This module | SEAL 4.1 |
| --- | --- | --- |
| A fresh ciphertext | about 26.7 | 26.6 |
| A rescaled square | 25.5 | 25.6 |
| A fourth power | 23.9 | 23.7 |
| A rotation or conjugation | 24.4 | 19.2 to 19.9 |

The rotations gain because key switching centres its digits.

A product with relinearization and rescaling takes about 7 ms on one core of a 12-thread desktop, against 3.2 ms for SEAL 4.1.

## std/crypto/fhe/tfhe

Each ciphertext encrypts one bit and every binary gate is evaluated on them and refreshed.

### TFHE ciphertexts

A `Ciphertext` is a learning-with-errors sample of 32-bit torus elements (`x / 2^32` of a turn) under a binary key, with `true` the phase `1/8` and `false` the phase `-1/8`.

`encrypt` and `decrypt` read the sign of the phase. `phase` gives it, and `constant` makes a bit anyone can read.

### TFHE parameters

`Parameters` names these and the two standard deviations:

| Name | Holds |
| --- | --- |
| `n` | The mask size. |
| `N` | The accumulator degree. |
| `l` | The gadget digits and their bits. |
| `t` | The key-switching digits and their bits. |

`context` rejects:

| Input | Rejected when |
| --- | --- |
| Degree | It is not a power of two from 2 to 2^15. |
| Mask | It is empty. |
| Gadget | It is empty, has digits wider than 16 bits or reaches below 32 bits. |
| Key-switching digits | They are missing or reach below 31 bits. |
| Deviation | It is negative or not finite. |
| Gadgets and degrees | Their exact products could wrap the prime (`Error.ProductOverflow`). |

`default_parameters` is the library's `default_128bit_gate_bootstrapping_parameters`: `n = 630`, `N = 1024`, `l = 3` digits of 7 bits, `t = 8` digits of 2 bits and deviations `2^-15` and `2^-25`, which the library estimates at 129 bits of security. Other parameters are not checked against an estimate.

### TFHE keys

`generate` returns a `SecretKey` and a `CloudKey`. The cloud key holds one ring encryption of each bit of the mask key, kept in transform form (about 124 MB at the defaults), and the key that switches the accumulator's key back to the mask key.

### TFHE gates

| Function | What it does |
| --- | --- |
| `apply` | Evaluates `And`, `Or`, `Nand`, `Nor`, `Xor`, `Xnor` and the four gates with one negated input (`AndNy`, `AndYn`, `OrNy`, `OrYn`) as a linear combination of the inputs plus an offset of an eighth or a quarter of a turn followed by one bootstrapping, so the output noise does not depend on the inputs'. |
| `not` | Negates a ciphertext without bootstrapping. |
| `mux` | Selects between two bits with two bootstrappings and one key switch. |

### TFHE bootstrapping

A bootstrapping:

1. Reduces the mask to multiples of `1/2N`.
2. Rotates the accumulator `X^b` by one controlled multiplexer per mask element, an external product of the gadget digits of the accumulator with the bootstrapping key.
3. Extracts the constant coefficient and key switches.

### TFHE exact arithmetic

The products of a digit polynomial and a key polynomial are computed exactly modulo one 60-bit prime with a number-theoretic transform from `std/crypto/fhe/ring`. The module sums the products of all digit rows in 128 bits before one reduction, so no floating-point rounding enters the noise and the measured variances equal the formulas to within a few percent.

### TFHE timings

A gate takes about 0.25 s on one core and generating the cloud key about 1.3 s.

## std/crypto/jose

`asymmetric_algorithms` lists the public key algorithms and `token_kid` reads the key id from a token header. `thumbprint` returns the RFC 7638 SHA-256 thumbprint of a `Jwk`. `sign_flattened` and `verify_flattened` write and check the flattened JSON serialization of RFC 7515 section 7.2.2 for a `Protected` header (`alg`, `kid`, `nonce`, `url` and an embedded `jwk`), as ACME uses it.

## std/crypto/password

`verify` handles all of them and `$2a$` and `$2y$` bcrypt strings. Argon2id hashes with `std/crypto/blake2b`.

For `/etc/shadow`, `verify` reads the strings libxcrypt's `crypt(3)` writes: `$5$` and `$6$` SHA-crypt with an optional `rounds=` field, and `$y$` yescrypt in its `j` (read-write), `.` (scrypt) and `/` (write-once) flavors, refusing a yescrypt string that would need more than 1 GiB. `hash_sha512_crypt` and `hash_yescrypt` write them with a random salt.

## std/crypto/sasl

`start` takes the user, the prepared secret and a nonce. `first` returns the client-first-message. `final` turns the server-first-message into the client-final-message with its proof. `verify` checks the server signature in constant time. Failures are `Malformed`, `Unsupported` (a mandatory extension) and `Rejected`. `std/sql/postgres` runs SCRAM-SHA-256 and `std/mongo` SCRAM-SHA-256 and SCRAM-SHA-1 through it. The caller prepares the secret.

## std/crypto/der

Reading: `reader`, `read`, `peek`, `done`, `expect`, `enter`, `finish`, `whole`, with `unsigned`, `small_int`, `boolean`, `bits`, `named_bits`, `oid` and `time` for the primitive types; it rejects indefinite and non-minimal lengths.

Writing: `encode`, `integer`, `sequence`, `set`, `octet_string`, `bit_string`, `utf8_string`, `object_identifier`, `time_value` and `trim_zeros`.

| Function | What it does |
| --- | --- |
| `object_identifier` | Writes from dotted text. |
| `time_value` | Writes a `UTCTime` for 1950 to 2049 and a `GeneralizedTime` for other years in 0001 to 9999. |
| `trim_zeros` | Drops the leading zero bytes of a big-endian magnitude. |
| `prime256v1` and `secp384r1` | Return the named-curve OIDs that `std/crypto/ecdsa` and `std/crypto/x509` share. |

`std/crypto/x509`, `std/crypto/rsa` and `std/crypto/ecdsa` build on it.

## std/crypto/x509

Certificate status: `Certificate` carries `subject_key_id`, `ocsp_urls`, `issuer_urls`, `crl_urls`, `ocsp_no_check` and `must_staple`.

| Function | What it does |
| --- | --- |
| `parse_crl`, `crl_lookup`, `verify_crl` and `crl_status` | Read revocation lists. |
| `ocsp_request`, `parse_ocsp` and `ocsp_status` | Build and check OCSP exchanges. |
| `verify_chain` | Applies a `Revocation` policy. |

See Certificate status.

Writing:

| Function | What it does |
| --- | --- |
| `generate_private_key` | Makes P-256, P-384 or Ed25519 keys (`KeyType`). |
| `public_key_info`, `private_key_der`, `sec1_private_key_der`, `private_key_pem`, `certificate_pem` and `pem_encode` | Encode keys and certificates. |
| `create_certificate` and `create_self_signed` | Write a v3 certificate from a `Template` with `Extension`s. |
| `create_request`, `parse_request` and `check_request_signature` | Write and check PKCS#10 requests. |

`Certificate.authority_key_id` holds the authority key identifier.

## std/acme

| Name | What it does |
| --- | --- |
| `connect` | Reads a directory and returns the `Client`. |
| `new_account`, `find_account`, `refresh_account`, `update_account`, `change_key` and `deactivate_account` | Manage an `Account`. |
| `new_order`, `get_order`, `get_authorization`, `respond`, `wait_authorization`, `wait_order`, `finalize` and `download_chain` | The steps of an order. |
| `obtain` and `csr_for` | `obtain` runs them with `Solvers` and returns the `Issued` certificate, and `csr_for` builds its request. |
| `revoke` and `revoke_with_key` | Revoke. |
| `renewal_info` and `renewal_id` | Implement RFC 9773. |
| `manager` | With `start`, `stop`, `run_once`, `renew_now`, `status` and `source`, keeps certificates renewed in a `Store` (`memory_store`, `directory_store`). |
| Challenge solvers | `challenges` with `put_token`, `drop_token`, `put_certificate` and `drop_certificate`, `http01_handler`, `serve_http01` and `tls_alpn_source`, `DnsRecords` and `PersistentDns`, and the helpers `key_authorization`, `dns01_name`, `dns01_value`, `http01_path`, `tls_alpn_server_name` and `tls_alpn_credentials`. |
| `dns_persist_name`, `dns_persist_value`, `dns_persist_strings`, `parse_dns_persist` and `account_hash_uri` | Write and read the `PersistRecord` of DNS-PERSIST-01. |
| `alpn_protocol` and `acme_identifier_oid` | The constants that name the TLS-ALPN-01 protocol and extension. |
| `describe` | Writes an `Error` as text; see ACME. |

## std/crypto/x509/fetch

`ocsp` sends a request with GET when it is under 255 bytes and POST otherwise, `crl` downloads a list, `fetcher` returns the `x509.Fetcher` that `Revocation.fetcher` takes and `client` returns the `*http.Client` they run on. It is a module of its own because `std/http` imports `std/tls`, which imports `std/crypto/x509`.

## std/time

Calendar and zone types live in the submodules below, one module per type, so `d.plus(span.days(3))` resolves to `date.plus`.

## std/time/duration

Constructors from `nanoseconds` to `hours`, and `from_seconds_f64`. Arithmetic: `plus`, `minus`, `times`, `divide`, `ratio`, `negate`, `abs`, with `try_` forms returning an optional. Plain forms panic on overflow. Also `round` and `truncate` with a `Rounding`, comparison, `to_string` (ISO 8601 `PT1H30M`), `friendly` (`1h 30m`) and `parse`, which reads both forms and fails at the offset of the first bad character.

## std/time/timestamp

`now`, `from_unix` in seconds, milliseconds or nanoseconds, `plus` and `minus` by `Duration`, `until` and `since`, `round`, `truncate`, RFC 3339 `to_string` and `parse` (offset required, `:60` read as `:59`), and `to_http_date` and `parse_http_date` (IMF-fixdate, RFC 850 and asctime).

## std/time/clock

A `Manual` clock moves with `advance` and `set`, and its sleep records the wait in `sleeps` and advances the clock without waiting, so code that asks for the time and waits is tested without sleeping.

## std/time/span

`Unit`, `Diff` (`largest`, `smallest`, `increment`, `mode`), `to_duration` and `from_duration`, `of_calendar` (calendar counts and a time duration), `of_elapsed` and `step_of` (rounding an exact elapsed time by a `Diff`), `round_fraction` (the rounding every difference shares), ISO 8601 `to_string` (`P1Y2M3DT4H`), `friendly` and `parse`.

## std/time/date

`Month`, `Weekday` and `IsoWeek`. `plus` and `minus` by `Span` with month-end clamping, `until` and `since` with a `Diff`, `next_weekday` and `on_or_next`, and `range`. ISO 8601 `to_string` and `parse` (calendar, ordinal and week dates). `format` and `parse_with` over a pattern.

## std/time/timeofday

`plus` fails past midnight and `plus_wrapping` wraps. Also `until`, `round`, `truncate`, `to_string`, `parse`, and pattern `format` and `parse_with`.

## std/time/zone

`Zone` is a shared value decoded once, from `named` (the host database, honouring `ZONEINFO`), `load_from` (any `fn(str) -> u8[]?`), `local` (`TZ`, `/etc/localtime`, `/etc/timezone`, then UTC), `from_posix`, `decode`, `utc` or `fixed`. `fixed_offset` returns the offset of a zone `fixed` made. Queries: `info_at`, `candidates` (`One`, `Overlap`, `Gap` for a civil time), `next_transition`, `previous_transition` and `transitions`.

## std/time/tzdata

The data is linked only when `source` or `names` is reachable.

## std/time/zoned

Built from a civil time with a `Disambiguation` (`Compatible`, `Earlier`, `Later`, `Reject`) or `strict`.

| Function | What it does |
| --- | --- |
| `plus` | By `Span` moves calendar units on civil dates and time units by elapsed time. |
| `plus_duration` | Is exact. |
| `until` and `since` | With a `Diff`. |
| `to_string` | RFC 9557 (`2026-11-11T10:00:00+00:00[Europe/London]`). |
| `parse` | With an `OffsetPolicy` for a written offset that disagrees with the zone. |

Also `start_of_day`, `hours_in_day`, `series` and `round`, `to_rfc3339` and `to_rfc2822`, and pattern `format` and `parse_with`.

## std/time/pattern

`compile_in` takes `Layouts` and so accepts `%x`, `%X` and `%c`, which `datefmt.layouts` fills from the standard patterns of a locale; `compile` rejects those three with `Unsupported`. Verbs: `%Y %y %C %m %d %e %j %b %B %a %A %u %w %U %W %V %G %H %I %M %S %f %N %p %P %z %:z %::z %Z %s %F %T %R %D %% %n %t`. Flags: `-`, `_`, `0` and `^`. Parsing is strict, and every error carries a byte offset.

## std/time/humanize

`ago_in` and `relative_in` take a `locale.Locale` and write the relative-time patterns of that locale.

## std/locale

| Name | What it does |
| --- | --- |
| `parse` | Reads a BCP 47 tag, canonicalises its aliases, keeps the `-u-` keys `nu`, `ca`, `co`, `hc` and `cu` as the `numbering`, `calendar`, `collation`, `hour_cycle` and `currency` fields of `Locale`, and loads the data of the locale and its parents. |
| `chain` | Lists the identifiers that serve a locale, most specific first and ending in `root` (`es-MX`, `es-419`, `es`, `root`; CLDR parent overrides such as `en-AU` to `en-001` apply). |
| `parse_with` | Takes the blobs of a generated `data.chain_<id>()` instead. |
| `root`, `tag`, `maximize` and `minimize` (likely subtags) | Complete the module. |
| `Error` | `Empty` or `Malformed(subtag)`. |

## std/locale/data

One blob per CLDR locale (1122 identifiers), the alias, likely-subtag, parent, numbering-system, currency, day-period and week tables, the plural rules, and the collation tables, each `embed`ded. `blob_by_id` reaches every blob; `chain_<id>()` (one per identifier) reaches only the blobs of one chain. A program is linked with the data its calls reach and no more.

## std/locale/decimal

`round_places`, `round_significant` and `round_increment` under a `Mode` (ceil, floor, expand, trunc and the five half modes).

## std/locale/plural

`Operands` (`i`, `v`, `f`, `w`, `t`, `e`) built by `operands_of_int`, `operands_of_text` or `operands`, then `cardinal`, `ordinal`, `category_of` and `range` (the category of a number range). `Category` is `Zero`, `One`, `Two`, `Few`, `Many` or `Other`, and `name` is its CLDR keyword.

## std/locale/numfmt

`currency` is an ISO 4217 code shown by `CurrencyDisplay` in a `CurrencySign` with `CurrencyUsage`.

| Type | Variants |
| --- | --- |
| `Style` | Decimal, percent or currency. |
| `Notation` | Standard, scientific, engineering or compact (`CompactDisplay` short or long). |
| `CurrencyDisplay` | Symbol, narrow symbol, code, plural name. |
| `CurrencySign` | Standard or accounting. |
| `CurrencyUsage` | Standard or cash digits. |
| `SignDisplay` | Auto, always, never, except-zero or negative. |
| `Grouping` | Auto, always, min2 or off. |

The digit options are `minimum_integer`, `minimum_fraction`, `maximum_fraction`, `minimum_significant`, `maximum_significant`, `rounding_mode` (a `decimal.Mode`), `rounding_increment` and `trailing_zeros`.

The numbering system comes from the `-u-nu` key of the locale. `localize_digits` and `decimal_separator` expose the digit and separator choices. Failures are `MissingCurrency`, `UnknownCurrency` and `InvalidDigits`.

## std/locale/datefmt

`Moment` is a civil date, a time of day and an optional offset and zone id, built by `from_date`, `from_time`, `from_datetime` and `from_zoned`.

`Options` takes:

| Option | Detail |
| --- | --- |
| `date_style` and `time_style` | `Length` full, long, medium or short. |
| `skeleton` | Matched against the available formats of the locale. |
| `hour_cycle` | `Auto` follows the `-u-hc` key and then the locale. |

| Function | What it does |
| --- | --- |
| `format` | Writes one value. |
| `format_range` | Writes an interval. |
| `pattern` | Returns the CLDR pattern. |
| `layouts` and `names` | Feed `pattern.compile_in`. |

`Error` is `EmptyRequest`, `MissingDate`, `MissingTime`, `MissingZone`, `InvalidSkeleton` or `NoLayout`.

## std/locale/collate

`Options` (`Alternate` non-ignorable or shifted, `Strength` primary to identical, `SecondaryOrder` for backwards secondary weights, `CaseFirst`, `numeric`), `root` and `for_locale` build a `Collator` from the root table and the CLDR tailoring of the locale. `compare`, `equal`, `compare_runes`, `sort_key`, `sort_key_runes` and `sort`.

## std/text

Integer parsing in any base from 2 to 36; correctly rounded float parsing, which allocates only to expand a decimal when digits beyond the 19th decide the rounding; `parse_bool`, `radix`, `hex`, hex encoding and `digit` and `hex_digit`, the value of one digit byte in a base or in hex; `append_hex` and `parse_hex_at`, which write and read a fixed number of hex digits; `fixed(x, places)`, which rounds half to even on the exact binary value.

### Text trimming and splitting

`trim` and its start, end, prefix and suffix forms; `fields`; `chars`; `join`.

| Function | What it does |
| --- | --- |
| `split` | Splits on a byte. |
| `split_str` | Splits on a string (an empty separator splits into code points). |
| `lines` | Drops a final empty line and `\r` before `\n`. |

### Text searching and rewriting

Searching: `index`, `last_index`, `contains`, `count`, `starts_with`, `ends_with`.

Rewriting: `repeat` and `replace`.

| Function | What it does |
| --- | --- |
| `upper` and `lower` | ASCII; return their argument when no byte changes. |
| `equal_fold` and `starts_with_fold` | Case-insensitive; allocate nothing. |

### Text widths and encoding

Widths: `char_count`, `from_char`, and `pad_left` and `pad_right` to a width in code points.

| Function | What it does |
| --- | --- |
| `append` and `append_char` | Append onto a `u8[]`. |
| `char_len` and `put_char` | Encode one code point into a buffer, and are the only UTF-8 encoder. |

Comma-separated token lists.

### Text cuts, quoting and other helpers

| Function | What it does |
| --- | --- |
| `cut` and `last_cut` | Return a `Cut` of `before`, `after` and `found`. |
| `strip_prefix` and `strip_suffix` | Return `str?`. |

Also `split_n`, `title`, `quote` and `unquote` (backslash, `\x`, `\u` and `\U` escapes, failing with `QuoteError`), `edit_distance` over code points, `wrap` to a width, `is_ascii`, `is_digits` and `reverse`.

### Text helpers

| Function | What it does |
| --- | --- |
| `is_scalar` | Tells whether a `u32` is a Unicode scalar value. |
| `lead_len` | Gives the length of the UTF-8 sequence a lead byte starts. |
| `trailing_backslashes` | Counts the backslashes that end a line. |
| `parse_u64` and `parse_uint` | Parse unsigned digits. |
| `digit_char` | Writes one digit. |
| `position(s, at)` | Turns a byte offset into a `Position` of line and column counted from 1. |
| `float32_bits` and `float32_from_bits` | Convert between an `f32` and its bits. |
| `float_decimal` | Writes a float in the decimal form that TOML and YAML share. |
| `float_diagnostic` | Writes a float in the form of CBOR's diagnostic notation. |
| `quote_ascii` | Writes a string as a double-quoted ASCII literal. |

## std/text/table

Widths use `unicode.text_width` and ignore ANSI sequences. `align` and `align_tabbed` align tab-separated cells like Go's `tabwriter`.

## std/text/number

Arithmetic is exact. `grouped_in`, `percent_in`, `compact_in` and `currency_in` take a `locale.Locale` and write the separators, digits, percent sign, compact words and currency patterns of that locale.

## std/uri

| Function | What it does |
| --- | --- |
| `escape` | Leaves the unreserved bytes. |
| `escape_form` | Writes a space as `+`. |
| `escape_target` | Encodes only the bytes a request target cannot hold and keeps existing escapes. |
| `escape_where` | Takes the set to keep. |
| `unescape` | Returns `str?` and is `none` on a bad escape. |
| `unescape_lossy` | Leaves a bad escape in place. |
| `unescape_form_lossy` | Also reads `+` as a space. |
| `encode_form` | Writes a list of `Field`s as `application/x-www-form-urlencoded`. |
| `parse_query` | Reads one into a `[str: str]` where the last value of a name wins. |
| `scheme_end`, `without_fragment`, `remove_dots` and `resolve` | Follow RFC 3986 section 5, working on strings. |

## std/builder

`write`, `write_bytes`, `write_byte`, `write_char`, `write_i64`, `write_f64`, `write_line`, `write_repeat`, `write_join`, `len`, `capacity`, `reserve`, `truncate`, `reset`, `to_str` and `to_bytes`.

## std/regex

`compile` and `compile_with` with `Flags`, `generated` (a pattern compiled into specialised code at build time), `is_match`, `find`, `full_match`, `find_all`, `replace_all` (with a closure receiving each `Match`), `split` and `split_n`. A match offers `start`, `end`, `text`, `group` and `span`, with byte offsets. `(?i)`, `\w` and `\b` follow Unicode, and `(?m)` ends a line at `\n`, `\r` or `\r\n`.

## std/unicode

| Area | Functions |
| --- | --- |
| Properties | `category`, `is_letter`, `is_upper`, `is_lower`, `is_digit`, `is_space`, `is_punct`, `is_identifier_start` and the other predicates; `east_asian_width`, `display_width` and `text_width`; `script`. |
| Case mapping | `to_upper`, `to_lower` and `to_title`, and `upper` and `lower` with SpecialCasing but without final-sigma or locale rules. |
| Segmentation | UAX #29 `graphemes`, `grapheme_count` and `grapheme_end`; `words`, `word_count` and `word_end`; `sentences`, `sentence_count` and `sentence_end`; and UAX #14 `line_breaks`, which lists the positions where a line may end and marks the mandatory ones, and `line_segments`. |
| UTF-8 helpers | `decode_rune`, `is_invalid`, `encode_rune`, `utf8_valid`, `utf8_to_valid`. |
| Byte order | `Order` is the byte order of UTF-16 and UTF-32, and `bom_order` reads the byte order mark of either width. |

| Data | Unicode version |
| --- | --- |
| Category, width and identifier data | 16.0.0 |
| Script, grapheme break and case data | 17.0.0 restricted to characters assigned in 16.0.0 |
| Word, sentence and line break data | 17.0.0 |

The tables are generated by `compiler/tablegen` and linked only when a function that reads them is reachable.

## std/charset

The encodings other than UTF-8, UTF-16 and UTF-32 follow the WHATWG Encoding Standard's decoders, encoders and indexes.

A byte a single-byte index leaves unassigned is:

| Function | Result |
| --- | --- |
| `decode` | `Malformed` |
| `decode_lossy` | U+FFFD |

`compiler/tablegen charset <index directory> std/charset` writes `tables.bin` and `tables.txt` from the single-byte `index-<name>.txt` files and one `<name>.bin` per multi-byte index (`jis0208`, `euc-kr`, `big5`, `gb18030` and `gb18030-ranges`): a pointer count, an entry width, the code point of each pointer and the pointers sorted by code point, which the encoders binary-search.

The index directory holds the WHATWG `index-*.txt` files of 2026-11-16, named by the canonical encoding name (`index-ISO-8859-2.txt`, `index-KOI8-R.txt`), and `index-IBM437.txt` and `index-IBM850.txt` in the same format, which WHATWG does not publish.

The tables total 344,528 bytes and are linked with any program that reaches `decode` or `encode`, since the label picks the table at run time.

EUC-JP, ISO-2022-JP and the other WHATWG single-byte encodings are not provided.

## std/html

A pull `Tokenizer` returning `StartTag`, `EndTag`, `Text`, `Comment`, `Instruction`, `Doctype` and `End` tokens, `parse`, `parse_fragment` and `parse_body` building a `Node` tree, tree navigation and editing, `inner_html`, `outer_html` and `reparsable_html`, CSS selector queries (`compile`, `query`, `query_all`, `matches`, `closest`), an allow-list sanitizer (`sanitize`, `clean`, `default_policy`), `unescape` for character references and `escape` for text and attribute values.

## std/markdown

| Name | What it does |
| --- | --- |
| `parse` | Returns a public `Block[]` tree of headings, paragraphs, fenced and indented code, raw HTML blocks, lists with a tight flag, tables with alignment, block quotes and thematic breaks. |
| `parse_inline` | Returns the `Inline[]` spans of a string. |
| `plain_text` | Extracts text from spans. |
| `Renderer` | Carries hooks for code blocks, code spans, headings, HTML blocks, inline HTML and tables that can call back into `render` and `render_inline`. |
| `render` and `to_html` | Produce the HTML `marked` writes with its default options. |

## std/math/stats

`sum` and `mean` (Neumaier compensated summation), `median`, `min`, `max`, sample and population `variance` and `stddev`, `quantile`, `percentile` and `quantiles` (linear interpolation between closest ranks, the default of `numpy.quantile`), `histogram` and `histogram_edges` (binning as `numpy.histogram`, last bin closed), and sample and population `covariance` and Pearson `correlation`.

## std/math/complex

The transcendental functions carry a `c` prefix because `exp` and `sqrt` are builtins.

## std/math/matrix

`lu` (partial pivoting) gives an `LU` with `lower`, `upper`, `permutation`, `lu_determinant` and `lu_solve`; `determinant`, `solve` and `inverse` build on it. `qr` (Householder reflections) gives a thin `QR`, and `least_squares` solves by it. A singular matrix is `Error.Singular`; a shape mismatch panics.

## std/math/fft

Power-of-two lengths use iterative radix-2 and other lengths Bluestein's algorithm.

## std/slices

Sorting:

| Function | What it does |
| --- | --- |
| `sort` and `sort_by` | Unstable and in place. They are a quicksort with median-of-three pivots that recurses into the smaller side and finishes runs of 16 or fewer with insertion sort. |
| `sort_stable` and `sort_stable_by` | A top-down merge sort. |
| `sort_by_key` | Stable. |

Also `reduce`, `count`, `find`, `find_index`, `first`, `last`, `equal`, `compare`, `starts_with`, `ends_with`, `unique` (adjacent duplicates), `flat_map`, `flatten`, `repeat`, `lower_bound`, `binary_search` (the first match) and `binary_search_by`.

| Function | What it does |
| --- | --- |
| `take`, `skip`, `chunks` and `windows` | Share the source's elements. |
| `partition` and `zip` | Work over `Pair[A, B]`. |
| `fill`, `swap`, `rotate`, `insert` and `remove` | Are in place. |

They chain as calls on the slice: `xs.sort()`, `words.map(fn(w) w.len).sum()`.|> slices.sort()`.

## std/os

### Operating system queries

Queries: `getcwd`, `chdir`, `hostname`, `uid`, `gid`, `euid`, `egid`, `ppid`, `executable`, `uptime`, `cpu_count`, `error_message`, `error_code` and `user_name`.

| Function | What it does |
| --- | --- |
| `error_message` | The system's message text for an error number, a `strerror` table on Linux and `FormatMessageW` on Windows. |
| `error_code` | The error number behind an `io.Error`. |

### Operating system accounts

Accounts read `/etc/passwd` and `/etc/group` and return `none`, or an empty slice, on Windows: `lookup_user` and `lookup_user_name`, `lookup_group` and `lookup_group_name` and `supplementary_groups` (the groups that list a login name), over `parse_passwd` and `parse_group`. Those two skip blank lines, comments and malformed lines.

`lookup_shadow` reads a user's `Shadow` entry from `/etc/shadow`, which needs root, over `parse_shadow`. `shadow_usable` refuses a locked or empty hash and an expired password or account.

### Operating system resource limits

`getrlimit` and `setrlimit` take a `Resource` and an `Rlimit`:

| Type | Holds |
| --- | --- |
| `Resource` | The sixteen Linux `RLIMIT_` resources. |
| `Rlimit` | `soft` and `hard` values where `none` is unlimited. |

They use `prlimit64` and fail with `Unsupported` on Windows.

### Operating system cgroup limits

`cgroup_limits` gives the `CgroupLimits` of the process: `cpu` (a `CpuLimit` of `quota`, `period` and `cpus`), `memory_max`, `memory_high` and `memory_current`. Here `none` is no limit, and a process outside a cgroup v2 hierarchy fails with `Unsupported`.

- On Linux it finds the group with `locate_cgroup` from the `0::` line of `/proc/self/cgroup` and the `cgroup2` mount that `parse_mountinfo` reads from `/proc/self/mountinfo` (decoding `\040`-style escapes and a mount root that is not `/`).
- Then, on Linux, `cgroup_limits_of` reads `cpu.max`, `memory.max` and `memory.high` from the nearest group that sets a limit, walking up to the mount's top group and passing over a missing file or `max`.
- On Linux it reads `memory.current` from the group itself.
- On Linux `parse_cpu_max` and `parse_memory_limit` reject other text as `Malformed`.
- On Windows the same call reads the job object of the process: a hard CPU rate cap as `cpu`, the smaller of the process and job memory limits as `memory_max`, and a process in no job fails with `Unsupported`.

### Operating system environment

`set_env` and `unset_env` are seen by `env`, `environment`, `process.find` and the environment `process.run` passes on.

### Operating system directories

`home_dir`, `cache_dir`, `config_dir`, `data_dir` and `state_dir` (XDG variables when absolute, otherwise under `$HOME`), and `temp_dir_path`.

## std/stdio

For `browser/wasm32` the host supplies the streams through `low.read` and `low.write`, and `exit` ends the run through `low.halt`, so a browser program reads input and sets its exit status without `std/os`.

## std/os/signal

`Signal` (`Hangup`, `Interrupt`, `Quit`, `IllegalInstruction`, `Trap`, `Abort`, `BusError`, `FloatingPoint`, `Kill`, `User1`, `SegmentationFault`, `User2`, `Pipe`, `Alarm`, `Terminate`, `Child`, `Continue`, `Stop`, `TerminalStop`, `TerminalInput`, `TerminalOutput`, `UrgentData`, `CpuLimit`, `FileSizeLimit`, `VirtualAlarm`, `ProfilingAlarm`, `WindowChange`, `IoReady`, `PowerFailure`, `BadSystemCall` and `Other(number)` for any other number), the one signal vocabulary of `std/process`, with `number`, `name`, `from_number` and `catchable`.

| Function | What it does |
| --- | --- |
| `notify` | Returns a droppable `Watch` (copyable `WatchHandle`) whose `signals` channel delivers the caught signals, with `wait_for`, `wait_timeout`, `wait_any` and `stop`. |

Also `ignore`, `reset`, `send` and `raise`. `Kill` and `Stop` cannot be caught and fail with `Invalid`.

## std/os/utmp

An `Entry` is one 384-byte record in glibc's x86-64 layout: its `Kind`, process id, terminal `line`, `id`, user, host, session, time and address. `encode` and `decode` convert a record, and `read_entries` reads a file of them.

| Function | What it does |
| --- | --- |
| `put` | Writes a record to a utmp file in place of the one with the same `id`, as `pututline` does, without glibc's lock. |
| `append` | Adds a record to a wtmp file. |
| `line_of` and `id_of` | Give a terminal's line and the id OpenSSH and `login` use. |

`utmp_path` and `wtmp_path` name the system's files, which only root may write.

## std/process

`Command` with arguments, an optional replacement environment, a working directory, `Inherit`, `Piped` or `Null` for each standard stream, an optional new process group, which is a job object on Windows and ends its child when the parent ends (`PR_SET_PDEATHSIG` with `SIGKILL` on Linux, `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE` on Windows), and `windowless`, which starts a Windows console program with no console window.

A `Command` also takes an `arg0` in place of the program's name, `Stdio.Descriptor(fd)` to give a stream one of the parent's descriptors, and on Linux a `Session` (`Same`, `New`, or `Terminal`, which makes its standard input the controlling terminal), `Credentials` to run as, a `root` directory to change into first and a `program_fd` to execute in place of a path. On Windows a `Session` other than `Same`, `Credentials`, a `root` and a `program_fd` fail with `Unsupported`.

| Function | What it does |
| --- | --- |
| `start` | Returns a droppable `Child`. When released, it kills and reaps an unwaited process and closes its pipes. |
| `run` | Feeds input and collects both outputs. |
| `find` | Searches `PATH`. |

`start_handle`, `adopt`, `wait`, `kill` (which takes a `std/os/signal` `Signal`; an `Exit` is `Exited(code)` or `Signaled(signal)` with the same type), and `wait_on`, `wait_exit_on` (waits without reaping, so the process group can still be signalled) and `kill_on` on the shared `ChildHandle`.

`success`.

## std/perf

On Windows every call fails with `Unsupported`.

| Function | What it does |
| --- | --- |
| `held` | Turns a `process.Command` into one whose process stops itself before it executes the program. |
| `await_held` | Waits for that stop. |
| `release` | Lets it go. |

So what is opened in between covers the run from its first instruction.

| Function | What it does |
| --- | --- |
| `counter` | Counts a `Source` (`cycles`, `instructions`, `cache_misses`, `branch_misses`, `cpu_clock`, `faults` or `switches`) over the process and everything it starts. |
| `count` | Reads it. |
| `available` | Fails with `Unsupported` or `Denied` when the machine does not count a source for the caller. |
| `sampler` | Takes samples at a `Pace` of `Hertz(n)` or `Events(n)`. |
| `drain` | Passes each `Record`: `Sampled` with a `Hit` (process, time, weight and the instruction addresses innermost first), `Mapped` with the `Mapping` of an executable file, `Forked`, `Executed`, `Lost` and `Other`. |
| `exited` and `usage` | Read the processor time, peak resident memory, page faults and context switches of an ended child before it is reaped. |
| `restrict_cpus` | Limits this process and its children to a mask of processors. |

## std/build

The compiler reads them from the source. When run as a program, each prints its declaration.

## std/buildinfo

| Constant | Holds |
| --- | --- |
| `revision` | The commit that `git rev-parse HEAD` prints in the program's project directory, and is empty when that fails. |
| `time` | The build time in seconds since the Unix epoch. It is read from `SOURCE_DATE_EPOCH` when that holds an integer and from the clock otherwise. |

The compiler writes both constants when the program imports the module. So a program that does not import it runs no `git` and does not depend on the time it was built. `low lsp` and `low doc std` see the empty placeholders.

## std/cdp

| Function | What it does |
| --- | --- |
| `launch` | Starts a headless Chromium with a disposable profile and connects to it. It returns a droppable `Browser` that shuts the Chromium down when released. Its `handle` is the shared `*BrowserHandle`. |
| `launch_handle`, `adopt`, `shutdown_on` and `open_page_on` | Take it. |
| `call` | Sends a command to the browser or a session. |
| `subscribe` | Each `subscribe` gets every other message as events. |
| `open_page` | Opens a page in its own browser context. |
| `close_page` | Disposes of that context. |

Every request of the page passes through `Handlers`, including those of cross-site frames and workers.

Also `navigate` and `shutdown`.

## std/filepath

On Linux and the browser it behaves as `std/path`.

On Windows `\` and `/` both separate and results are written with `\`. A volume is kept by `clean` and `dir`:

| Volume | Form |
| --- | --- |
| drive | `C:` |
| share | `\\server\share` |
| device | `\\?\C:`, `\\?\UNC\server\share` |

A share, a device path or a drive followed by a separator is absolute, while `C:x` and `\x` are not.

`join` adds no separator after a bare drive and drops the leading separators of later parts, so two parts never form a share.

The volume rules are the private `volume_length` in `volume_windows.low`. `volume_linux.low` and `volume_browser.low` give none.

## std/path

`matches` matches shell globs with `*`, `?`, `[...]` classes (ranges, `!` or `^` negation) and `\` escapes. No wildcard crosses a `/`.

## std/sync

| Type | Functions | Notes |
| --- | --- | --- |
| `WaitGroup` | `add`, `done`, `wait` | |
| `Once` | `run` | |
| `RwLock` | `read_lock`, `read_unlock`, `write_lock`, `write_unlock`, `try_read_lock`, `try_write_lock`, `reading`, `writing` | First in first out and writer-fair. |
| `Semaphore` | `acquire`, `try_acquire`, `release`, `available` | |
| `Cond` | `wait_cond`, `signal`, `broadcast` | |
| `Barrier` | `wait_barrier` | |
| `OnceValue[T]` | | |
| `ErrorGroup[E]` | | Its `go` blocks while `limit` tasks run, and its `join` returns the first error. |

All are built on channels, so a waiting green thread parks instead of spinning.

`lock_thread` and `unlock_thread` pin the calling green thread to its OS thread. This is for C libraries whose state belongs to one thread. The calls nest.

## std/retry

`delay`. A loop driver `start` with `next` (`reserve` then `commit`) and `at_least`. `run` over a `Step` (`Done`, `Retry`, `RetryAfter`, `Fatal`) gives an `Outcome`.

## std/ratelimit

`Window`, a sliding log. `Keyed`, a bounded per-key set of buckets with idle eviction. All take a `clock.Clock`.

## std/singleflight

`Expiring[V]`, a value refreshed at most once per interval.

## std/parallel

`each`, `chunks`, `try_each` and `map` return when every call has returned. They run on at most `cpus()` green threads, one of them the caller's, so a job may call them again. `cpus()` is 1 for `browser/wasm32`, where the calls run in order.

| Function | Calls | Notes |
| --- | --- | --- |
| `each(count, job)` | `job(i)` for every `i` in `0..count` | Hands out ranges of `count / (32 * cpus())` indices, at least 1. |
| `chunks(count, grain, job)` | `job(start, end)` for every range of `grain` indices | The last range is shorter when `grain` does not divide `count`. Use it for a job as small as one arithmetic step, which loops over its range itself. |
| `try_each[E](count, job)` | `job(i)` until one returns `some(e)` | Returns the error of the lowest failing index, whichever thread finishes first. Every index below it is called. |
| `map(xs, f)` | `f(x)` for every element | The result has the order of `xs`. |

A job is a closure and may write to an element of a slice it captures, as `map` does. Two calls that write one element, or change one captured variable, race: use `std/atomic` or `std/sync` there. A panic in a job ends the program.

## std/atomic

Every load is a locked operation, so a loop cannot hoist it. `Value[T]` swaps a heap pointer under a lock: `load_value`, `store_value`, `swap_value`, `update_value`, `snapshot_value` and `compare_and_swap_value` (compares pointer identity).

## std/rand

`pcg32`, `pcg64` and `chacha` produce an `Rng` that every function accepts. `chacha` is seeded from 32 bytes and produces the keystream of `std/crypto/aead`'s `chacha20_block`, computed by `chacha20_state`. `chacha_rng` seeds from entropy. `next32` draws 32 bits. Distributions: `normal`, `exponential`, `gamma`, `beta`, `poisson`, `binomial`, `geometric`, `zipf`, `weighted_pick`, `cumulative` with `pick_cumulative`, `sample` and `permutation`. Use `std/crypto` for secrets.

## std/csv

`parse` and `parse_with` a separator fail with the line of an unterminated or bare quote. `format`, `format_with` and `append_record` quote only fields that need it.

## std/dotenv

`parse` and `read` return a map from `NAME=value` lines. They allow `export`, blank lines, `#` comments (inline after whitespace in unquoted values), single quotes taken literally, and double quotes with `\n`, `\t` and `\r` escapes that may span lines. Errors carry the line.

## std/gpu

`run_on_cpu(job)` takes what `launch(kernel, count, arguments)` gives and calls the kernel's CPU build once for every index below `count`, through `parallel.each`. It returns when every call has returned. The calls have no order, so a kernel writes only elements that no other invocation reads or writes. An out-of-range access panics, as it does in any function.

## std/flag

`get`, `enabled`, `number`, the remaining arguments, and `usage`.

## std/log

It writes no timestamps, so journald and other collectors add their own. `with_prefix` nests a prefix. A `Logger` also carries:

| Carries | Holds |
| --- | --- |
| attributes | `str_attr`, `int_attr`, `float_attr`, `bool_attr`, `with_attrs`, and the `_with` level forms |
| a `Format` | `Text` (`key=value`, quoting what needs it) or `Json` (one object per line) |
| a `Sink` | a descriptor, an `io.Stream`, a line closure, a `Record` handler, or a `Destination` function that can fail, set with `with_destination` |
| timestamps | optional RFC 3339 timestamps |
| a level | from `parse_level` or the `LOG_LEVEL` variable |

Each record is one `write`, so lines from green threads do not interleave.

A failed write or failed `Destination` is passed to `Logger.on_error` (set with `with_error_handler`), which ignores it by default.

`std/log/syslog` and `std/log/journald` attach as destinations.

## std/log/syslog

`format` writes a `Message` (a `Facility`, a `Severity`, an optional timestamp, hostname, app name, process id, message id, structured data as `Element` and `Param` values and the text) as `<PRI>1 TIMESTAMP HOSTNAME APP-NAME PROCID MSGID STRUCTURED-DATA MSG`.

| Part | Rule |
| --- | --- |
| Timestamp | RFC 3339 UTC. |
| Header fields | Cut to the lengths of section 6 and to printable ASCII. |
| An empty field | Written as `-`. |
| Structured-data values | Escaped as section 6.3.3 says. |
| Text that is not ASCII | The UTF-8 byte order mark comes before it. |

| Function | Does |
| --- | --- |
| `message` | Makes a `Message` from a `log.Record` and puts the attributes in one structured-data element. |
| `severity_of` | Maps a level to a severity. |
| `open` | Returns a droppable `Syslog` for a `Transport`. |

`Transport` values:

| Transport | Notes |
| --- | --- |
| `UnixDatagram` | The default, `/dev/log`. |
| `UnixStream` | |
| `Udp` | RFC 5426. |
| `Tcp` and `Tls` | RFC 6587 and RFC 5425, each message framed by octet counting, which `octet_counted` writes. |

The first `send` makes the connection, and so does the first one after a failure. A failed send returns the error and the message is lost. Sends from green threads are serialised.

`send`, `send_record`, `close` and `attach`. `attach` returns a `log.Logger` that sends every record through the `Syslog`.

## std/log/journald

`encode` writes `Field` values as datagram payload:

| Field | Written as |
| --- | --- |
| A field with a value with no newline | `NAME=value` and a newline. |
| A field with a value with a newline | The name, a newline, the length as a 64-bit little-endian integer, the value and a newline. |
| A field whose name is not valid (`valid_name`) | Left out. |

`fields` makes the fields of a `log.Record`:

- `PRIORITY` (`priority_of`)
- `MESSAGE`
- `SYSLOG_IDENTIFIER`
- one field for each attribute, named by `field_name`, so an attribute `code_file` or `code_line` becomes `CODE_FILE` or `CODE_LINE`

`open` returns a droppable `Journal` that sends to `/run/systemd/journal/socket` by default over a Unix datagram socket, opened by the first `send` and again after a failure.

An entry too large for a datagram goes as a sealed in-memory file passed with `SCM_RIGHTS`, as the protocol describes, and fails with `Unsupported` on Windows.

`send`, `send_record`, `close` and `attach`.

## std/term

| Name | Description |
| --- | --- |
| `enter_raw` | Returns a droppable `Raw` that restores the terminal when released. |
| `Key`, `read_key`, `parse_keys` | `read_key`: a lone escape resolves after 50 ms. |
| ANSI helpers | `fg`, `bg` with 16, 256 and RGB colours, `bold`, `dim`, `underline`, cursor movement, `clear_screen`, the alternate screen, `strip` and `visible_width`. |
| `colour_enabled` | Honours `NO_COLOR`, `CLICOLOR_FORCE` and `TERM=dumb`. |
| `open_pty` | |

On Windows it switches a console to virtual terminal processing, so the escape sequences print as colour.

## std/mime

`sniff` recognises content by its magic bytes or its leading text and falls back to plain text or `application/octet-stream`.

## std/container/lru

All run in constant time over index-linked entries.

## std/archive

Names are cleaned and rejected when absolute, containing `..` or NUL, or passing through a symbolic link below the destination. Files are created exclusively without following links. The extractors share its `Stamp` and `apply_stamps` for directory times, `write_from` for file bodies and `walk` for the sorted tree walk of `create`. The tar and cpio readers share its `Input`, which counts every byte against the total limit.

## std/archive/tar

A streaming `Reader` (`next`, `read_body`, `read_entry`, `list`) with limits on header, entry and total size. A `Writer` (`add_file`, `add_dir`, `add_symlink`, `add_hardlink`, `finish`) that uses pax records for long or non-ASCII names. gzip through `read_gzip` and `write_gzip`. `extract`, `extract_gzip`, `create` and `create_gzip` over the filesystem strip setuid, setgid and sticky bits.

## std/archive/cpio

Hard links store their data with the last link in newc and CRC. The binary format fails with `Unsupported`.

## std/archive/zip

Written with `add_file`, `add_dir`, `add_symlink` and `finish`. `finish` emits zip64 records when a count or size needs them.

`begin_entry` returns an `EntryWriter` that takes one file entry in pieces through `write_entry` and completes it with `end_entry`: the data goes through the streaming raw deflate encoder or is stored as it arrives, so the writer keeps only the encoder state, a running CRC-32 and the byte counts, and a `stream_writer` sink receives it before the entry ends.

The local header sets general purpose flag bit 3 with the CRC-32 and sizes zero and a data descriptor follows the data, so the descriptor width is fixed when the entry begins:

| Condition | Descriptor |
| --- | --- |
| `force_zip64` was called first | 8-byte sizes with a zip64 extra field in the local header. |
| Otherwise | 4-byte sizes and `write_entry` fails with `TooLarge` at `0xFFFFFFFF` bytes. |

While an entry is open `add`, `begin_entry` and `finish` fail with `Invalid`.

`extract` works over the filesystem. Encrypted entries and other methods fail with `Unsupported`.

## std/testing

Assertions `equal`, `not_equal`, `equal_slice`, `is_true`, `is_false`, `is_some`, `is_none`, `contains`, `contains_item`, `close_to`, `unexpected`, `must` and `expect_error` render values of any type. `table` and `table_of`.

`run` and `run_with` run each test on its own green thread with a timeout, filter with `-run`, print with `-v` and exit with status 1 on failure. `Settings.timeout_ms` of `0` sets no limit, for a parent process that enforces the timeout. `Test` carries the `file` and `line` of its function, which a generated list fills.

| Flag | Effect |
| --- | --- |
| `run` with `-list` | Prints `name`, `file` and `line` separated by tabs for each test. It exits with status 0 without running anything. |
| `run` with `-one <name>` | Runs only the top-level test of that exact name and prints its report without a summary. |

`run` with `-one <name>` exits as follows:

| Result | Status |
| --- | --- |
| The test passed | 0 |
| The test failed | 1 |
| The test was skipped | `skipped_status` (3) |
| No test has that name | 2, after printing `testing: no test named <name>` |

`low test` runs a test program through these two flags, one process per test.

`-inputs <file>` with `-one` creates the file and starts `sys.record_inputs` on it just before the test runs, so the runner's own reads are not logged. A file that cannot be created leaves the test running without a log.

`bench`, `bench_for` and `blackhole`. `golden` rewrites its file when `Settings.update_golden` is set, which `-update` does. `check_property`, `check_shrunk` and `check_ints` print the seed and shrink integers.

## std/arrow

### Arrow arrays and types

`Array` holds a type, a length, a validity bitmap, value buffers, children and a dictionary for null, bool, 8 to 64-bit integers, float16, float32, float64, decimal128, date, time, timestamp, duration, interval, binary, utf8 and their large forms, fixed-size binary, list, large list, fixed-size list, struct, map, dense and sparse union and dictionary types. `Schema` and `Field` carry key and value metadata.

Decimal256, view types, run-end encoding, big-endian data and the version 4 union layout are not supported, and reading them fails with `Unsupported`.

### Arrow buffer alignment

Buffers are `u8[]`, so an array in memory is not 64-byte aligned.

| Side | Alignment |
| --- | --- |
| Writer | Aligns the metadata and every body buffer to `WriterOptions.alignment` (8, 16, 32 or 64 bytes, default 8). |
| Reader | Accepts any alignment. |

### Arrow builders and values

`builder` with `append_*`, `child`, `end_list`, `end_struct`, `append_union` and `finish` builds arrays. A dictionary builder interns text, bytes and integers. `finish` validates them. `validate` checks offsets, lengths, null counts and type ids of any array.

`int_at`, `uint_at`, `bool_at`, `float_at` (half floats widen to `f64`), `bytes_at`, `str_at`, `decimal_text`, `list_range`, `union_slot` and `dictionary_index` read values. `diagnostic` prints an array on one line.

### Reading Arrow IPC

`stream_reader`, `stream_bytes`, `next_batch`, `read_batches`, `open_file`, `open_file_bytes`, `read_batch` and `read_file_batches` read the IPC stream and file formats, including the continuation marker, the pre-0.15 framing, dictionary batches and deltas, and LZ4 frame and Zstandard compressed bodies.

### Arrow input limits and errors

| Limit | Applies |
| --- | --- |
| `ReaderOptions` | Caps the metadata and body sizes. |
| Offsets and lengths in the hand-read Flatbuffers metadata | Every one is checked against its buffer. |
| Nesting | Limited to 64 levels. |

Truncated or corrupt input returns an `Error` (`Invalid`, `Truncated`, `TooLarge`, `Unsupported`, `Mismatch` or `Io`) and does not panic or loop.

### Writing Arrow IPC

`stream_writer`, `file_writer`, `write_batch`, `close`, `write_stream` and `write_file` write both formats, with `Compression` of `Uncompressed`, `Lz4` or `Zstd` per buffer. A buffer that does not shrink is stored as it is.

| Writer | A dictionary whose values changed |
| --- | --- |
| Stream writer | Resends it. |
| File writer | Fails with `Mismatch` because the file format cannot replace one. |

Dictionary deltas are read but never written.

## std/avro

`parse` reads a schema (primitives, records, enums, arrays, maps, unions, fixed, named references, namespaces, aliases, defaults and the logical types decimal, uuid, date, time-millis, time-micros, the timestamp and local-timestamp types and duration) into a `*Schema` graph.

| Function | Does |
| --- | --- |
| `canonical`, `fingerprint` (CRC-64-AVRO), `fingerprint_sha256` and `fingerprint_md5` | Give the parsing canonical form and its fingerprints. |
| `to_json` | Writes a schema back. |
| `Value` | The dynamic datum, with a logical variant for each logical type. |
| `decode` and `decode_with` | Read binary data. `Options` limit the length, element count and depth the input may declare, so a hostile length or a zero-byte element count cannot allocate or loop. |
| `encode` | Writes binary data, failing with `Mismatch` and the path of a value that does not fit. |
| `decode_resolved` and `resolve` | Apply schema resolution: promotion, aliases, defaults, enum defaults and union matching. |
| `serialize[T]`, `deserialize[T]`, `to_value` and `from_value` | Go through `std/serde`. |
| `encode_single`, `decode_single` and `SchemaStore` | Implement single-object encoding. |
| `diagnostic` | Prints a value on one line. |

Object container files:

| Function | Does |
| --- | --- |
| `file_writer` and `open` | Write and read object container files as a stream over `std/io`, with the codecs null, deflate, snappy (with its CRC-32), zstandard, bzip2 and xz. |
| `append` or `write_value` | Adds values. |
| `finish` | Closes the last block. |
| `next`, `read_all` or `read_values[T]` | Yield them. |

A writer takes a fixed sync marker for reproducible files. A reader checks the magic, sync markers, block sizes and checksums.

Every decoding failure is an `Error` carrying the byte offset.

## std/parquet

### Reading Parquet files

| Function | Does |
| --- | --- |
| `open_bytes` and `open` | Read the footer into `Metadata` (schema, row groups, column chunks, encodings, statistics, key and value metadata). `open` reads a file by seeking to each column chunk and keeps no file open. |
| `read_row_group`, `read_batches` and `read_column` | Return Arrow arrays. |
| `columns` and `arrow_schema` | Describe the leaf columns and the Arrow schema (`PARQUET:field_id` becomes field metadata, an unknown logical type reads as `null`). |
| `read_rows` | Returns `Value` rows. |
| `diagnostic` | Prints a `Value`. |
| `select_row_groups` | Keeps the row groups whose integer statistics can hold a range. |

A field list projects columns and reads only their chunks.

### Parquet types, encodings and codecs

| Kind | Names |
| --- | --- |
| Physical types | boolean, int32, int64, int96, float, double, byte array and fixed-length byte array |
| Logical types | string, enum, UUID, JSON, BSON, float16, date, integers, decimals, times and timestamps |
| Nested types | Lists (the three-level form and the older repeated forms), maps and structs of any nesting through repetition and definition levels |
| Encodings | PLAIN, the RLE and bit-packed hybrid, PLAIN_DICTIONARY and RLE_DICTIONARY, DELTA_BINARY_PACKED, DELTA_LENGTH_BYTE_ARRAY, DELTA_BYTE_ARRAY, BYTE_STREAM_SPLIT and RLE booleans, in data pages v1 and v2 and dictionary pages |
| Codecs | UNCOMPRESSED, SNAPPY, GZIP, ZSTD, BROTLI, LZ4_RAW and the Hadoop-framed LZ4 (read only) |

### Parquet checks and limits

| Item | Behaviour |
| --- | --- |
| Page CRC | Checked unless `ReaderOptions.verify_checksums` is off. |
| Statistics | Trusted when the file declares a type-defined column order, or for the older fields only on boolean, signed integer and float columns. |
| Encrypted files and LZO | Fail with `Unsupported`. |
| Bloom filters and page indexes | Ignored. |
| `ReaderOptions` | Caps the footer, page and value count. |
| The hand-written Thrift compact reader | Limits nesting and every length. |

Damaged input returns an `Error` (`Invalid`, `Truncated`, `TooLarge`, `Unsupported`, `Mismatch`, `Checksum` or `Io`) and does not panic or loop.

### Writing Parquet files

`write_bytes`, `write` and `write_rows` write a file from Arrow batches or `Value` rows. `WriterOptions` sets:

| Setting | Detail |
| --- | --- |
| Codec and level | SNAPPY by default; the Hadoop LZ4 and LZO are not written. |
| Dictionary encoding | With a fallback to PLAIN when the dictionary page would exceed `dictionary_page_size`. |
| Data page version | 1 or 2. |
| Page size | |
| Rows per row group | |
| Statistics | |
| Page CRCs | |
| Encoding per column | PLAIN, RLE, DELTA_BINARY_PACKED, DELTA_LENGTH_BYTE_ARRAY, DELTA_BYTE_ARRAY, BYTE_STREAM_SPLIT or RLE_DICTIONARY. |

The footer declares type-defined column orders. The statistics hold the minimum and maximum (NaN excluded, unsigned and decimal orders respected) and the null count.

Arrow dates in milliseconds, times and timestamps in seconds, durations, intervals and unions are not written (`Unsupported`). A null in a non-nullable field is `Mismatch`.

## std/mongo

`connect` takes a `mongodb://` string and `connect_with` an `Options`; both return a droppable `Client` whose copyable view is `ClientHandle`.

`database` and `collection` give the handles that `find`, `find_one`, `aggregate`, `count_documents`, `estimated_document_count`, `distinct`, `insert_one`, `insert_many`, `update_one`, `update_many`, `replace_one`, `delete_one`, `delete_many`, `find_one_and_delete`, `find_one_and_replace`, `find_one_and_update`, `bulk_write`, `create_index`, `create_indexes`, `drop_index`, `list_indexes`, `create_collection`, `drop_collection`, `drop_database`, `list_collection_names`, `list_database_names` and `run_command` take.

| Name | Description |
| --- | --- |
| `Cursor` | Reads with `next` and `all` and is released with `close_cursor`. |
| `Session` | Comes from `start_session` and carries `start_transaction`, `commit_transaction`, `abort_transaction` and `with_transaction`. |
| `text_of`, `int_of`, `document_of` and the other builders | Work on `bson.Field[]`. |
| `get_text`, `get_int` and the other getters | Work on `bson.Field[]`. |
| `Error` | Separates connection, authentication, server (with its error labels), write and write-concern failures. |

## std/big

Parsing and printing work in bases 2 to 36. Printing divides the limbs, packed into 64-bit words, by the largest power of the base below 2^64 with a precomputed reciprocal.

- `add`, `sub`, `mul` (Karatsuba), `divmod`, `divmod_floor`, `divmod_euclid`, `pow`, `modpow` (Montgomery for odd moduli), `mod_inverse`, `gcd`, `lcm`, `isqrt`.
- Shifts and bitwise operators with two's complement semantics.
- `is_probable_prime`, `next_prime`, `random_below`, `to_f64`, `from_bytes` and `to_bytes`.

Division by zero is an error. Not constant time.

## std/redis

It has a droppable `Client` with a `ClientHandle`, `connect` with AUTH, SELECT and HELLO 3, `command`, typed helpers for strings, hashes, lists, sets, sorted sets and `publish`, pipelining, `transaction`, pub/sub through a channel usable in `select`, command timeouts, and limits on bulk size and nesting.

## std/kafka

It has a `Client` that green threads share and `close` ends, `connect` with bootstrap addresses, `ApiVersions` negotiation and SASL PLAIN, SCRAM-SHA-256 and SCRAM-SHA-512, and one connection per broker that the metadata names.

A call fails with `Unsupported` when the broker's range lacks the version the client speaks, so a broker without `CreateTopics` still serves `produce` and `fetch`.

| Function | Does |
| --- | --- |
| `metadata`, `list_topics`, `create_topic` and `delete_topic` | Manage topics. |
| `produce` | Appends one record batch of format 2, with `acks`, and gzip, Snappy, LZ4 (independent blocks, as brokers require) or Zstandard compression. |
| `send` | Places one record by `partition_for`, the murmur2 placement of the Java client for a key, or in turn without one. |
| `fetch` | Reads from an offset, checks every batch CRC-32C, drops the records below the offset and gives the offset to fetch next across marker and compacted batches. |
| `list_offsets` | Finds `earliest`, `latest` or the offset of a time. |
| `encode_batch` and `decode_batches` | Expose the record batch codec. |

A request is retried up to `Options.retries` times after a leader change, a lost connection or another retriable broker code. The metadata is refreshed first. A closed client fails with `Closed` without retrying.

Consumer groups:

| Function | Does |
| --- | --- |
| `subscribe` | Joins a consumer group through its own connection to the coordinator. It divides the partitions by the Java client's range assignor and resumes committed offsets or follows `Reset`. |
| `poll` | Heartbeats, rejoins after a rebalance or an expired session, follows leader changes and returns `Message`s. |
| `commit`, `seek`, `position`, `assignment` and `unsubscribe` | Complete it. |
| `ConsumerOptions.auto_commit_interval` | Commits from `poll`. |

These are not provided:

- Transactions and the idempotent producer
- the cooperative and sticky assignors
- static membership
- the new consumer group protocol (KIP-848)
- background batching of `produce`

## std/nats

It has:

- a droppable `Client` with a `ClientHandle`
- `connect` with a token, a user and password or an NKey seed that signs the server nonce
- the TLS upgrade when the server sends `tls_required`
- `connect_tls` for a server that handshakes first
- `connect_stream`
- `publish` and `publish_with` (reply subject and headers)
- `subscribe` and `queue_subscribe` through a channel usable in `select`
- `unsubscribe` and `unsubscribe_after`
- `request` over one shared inbox with a timeout and `NoResponders`
- `flush`, `drain`
- answers to the server `PING` and a client ping interval with a missed-pong limit
- subject and header validation, `max_payload` enforcement and limits on control lines and message size

Reconnection with server lists, JetStream, key/value and object stores, the WebSocket transport and JWT credential files are not included.

## std/mqtt

It has a droppable `Client` with a copyable `ClientHandle`.

| Function | Does |
| --- | --- |
| `connect`, `connect_tls` and `connect_stream` | Take an `Options` that picks the version, credentials, will, session, keep alive, `receive_maximum`, `max_packet_size` and topic aliases. |
| `publish` and `publish_async` with `wait_publication` | Run QoS 0, 1 and 2 in both directions. |
| `subscribe`, `subscribe_with` and `unsubscribe` | Take `Filter`s with the MQTT 5 options. |
| `reconnect` | Resumes a session that outlives the connection (`clean_start` false in MQTT 3.1.1, a session expiry above 0 in MQTT 5). It resends unacknowledged publishes with dup, completing them with `SessionLost` when there is no session to resume. |
| `encode`, `decode` and `read_packet` | Expose the codec. |
| `valid_topic_name`, `valid_filter` and `matches` | Validate and match topics. |
| `close_client` | Is explicit and idempotent from `Client` and `ClientHandle`. |

Incoming messages arrive on a channel usable in `select` that holds back the connection's reads while it is full. The end of a connection arrives on `disconnected`.

MQTT 5 properties, server DISCONNECT and AUTH through an `Options.authenticate` callback are supported. A connection whose broker stops reading ends through `Options.timeout` or the keep alive.

WebSocket, automatic reconnect loops and client-initiated re-authentication are not provided.

## std/amqp

It has a droppable `Connection` from `connect`, `connect_tls` or `connect_stream` taking an `Options` that picks:

- the PLAIN, AMQPLAIN or EXTERNAL mechanism
- virtual host
- connection name
- `heartbeat` (sent at half the interval, the connection ending after two silent intervals)
- `frame_max`
- `channel_max`
- timeouts
- the consumer buffer

| Function | Does |
| --- | --- |
| `open_channel` | Gives a `Channel` that green threads share. Calls on it run one after another. A publish is written whole. A delivery is settled from any thread. |
| `declare_exchange`, `declare_queue`, `bind_queue`, `bind_exchange`, `purge_queue` and their deletes | Take option structs. |
| `publish` | Splits the body to the agreed frame size and takes the properties, `mandatory` and `immediate`, with returned messages on `Channel.returns`. |
| `confirm_mode`, `wait_confirm`, `wait_confirms` and `publish_confirmed` | Track publisher confirms by sequence number. |
| `Nacked` | Reports `basic.nack`. |
| `consume` | Gives a `Consumer`. Its `deliveries` channel is usable in `select` and holds back the connection's reads while it is full. |
| `get`, `ack`, `ack_through`, `nack`, `nack_through`, `reject`, `recover`, `qos` and the `tx_` calls | Cover the rest of the `basic` and `tx` classes. |
| `encode_method`, `decode_method`, `encode_frame`, `read_frame`, `encode_header` and `encode_fields` | Expose the codec. |

| Event | Result |
| --- | --- |
| A `connection.close` or `channel.close` from the broker | Ends the pending call with `ConnectionClosed` or `ChannelClosed` and its code. |
| `basic.cancel` from the broker | Closes the consumer's `deliveries`. |
| `channel.flow` | Is answered and shown by `flowing`. |
| RabbitMQ's `connection.blocked` and `connection.unblocked` | Arrive on `Connection.notices` and through `blocked`. |

Reconnecting and topology recovery are not provided.

## std/smtp

`connect`, `connect_starttls` and `connect_tls`, EHLO extensions, AUTH PLAIN, LOGIN, XOAUTH2 and CRAM-MD5 (plaintext authentication is refused unless `insecure_auth`), `send` with per-recipient results, dot-stuffing, pipelining when advertised, and errors separating 4xx from 5xx.

## std/template

`{{ expr }}`, `if`, `for`, `include`, `block` and `extends`, `let`, trim markers, filters (built in and registered), HTML autoescaping, depth and output limits, and errors with line and column. A `Set` is read-only while rendering, so green threads share it.

## std/multipart

A streaming `Reader` with limits and a spill sink for large parts, RFC 2231 and 5987 parameters, `Writer` (`write_field`, `write_file` and `write_form_part`) and `Form`; `std/http` reads and writes them with `multipart_reader`, `multipart_form`, `set_multipart` and `with_multipart`.

## std/base64

`encode`, `encode_unpadded` and `encode_url`, and `decode`, `decode_unpadded` and `decode_url`, fix the alphabet and padding. Every decoder rejects non-zero trailing bits and a byte outside the alphabet.
