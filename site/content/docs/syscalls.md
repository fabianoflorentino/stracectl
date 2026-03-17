---
title: "Syscall Reference"
description: "Built-in reference for the ~50 Linux syscalls tracked by stracectl, organised by category."
weight: 5
---

stracectl ships a built-in knowledge base for the most common Linux syscalls.
This reference is surfaced in three places:

- **TUI detail overlay** — press `Enter` or `d` on any row
- **Web detail page** — click any row or navigate to `/syscall/<name>`
- **Help overlay** — press `?` in the TUI

The table below documents each canonical syscall: its category, signature,
arguments, return value, common error codes, and diagnostic notes.
Aliases (legacy or variant names that are silently normalised to the canonical
entry) are listed for each syscall where they exist.

---

## Limitations

- Not every Linux syscall is covered by the built-in knowledge base; unknown
	 syscalls are shown with a generic entry.
- When using the classic `strace` text parser, very complex or non-standard
	 argument formats can be mis-parsed; prefer the eBPF backend when accurate
	 structured data and low overhead are required.
- Latency values measured via `strace -T` include user/kernel scheduling and
	 may be less precise than kernel-level eBPF timings. Treat `latency_ns` as
	 indicative rather than absolute unless using the eBPF backend.
- Timestamps are produced by the server process; when comparing traces across
	 hosts verify clock sync (NTP) or prefer aggregated metrics.
- Attaching to processes with `ptrace` may require privileges and can be
	 restricted by kernel settings (e.g., YAMA ptrace_scope). See the Security
	 and Kubernetes docs for recommended setups.

## How to interpret results

Use the dashboard and API fields together to triage issues:

- **High error rate (ERR% ≥ 50%)**: a red row indicates functionality is
	 failing — inspect `recent_errors` and the `error_breakdown` for the syscall
	 via `/api/syscall/{name}` to find common errno values and samples.
- **High average latency (AVG ≥ 5 ms)**: a yellow row flags kernel time that
	 may indicate I/O stalls or contention. Check P95/P99 to see whether the
	 behaviour is due to outliers or sustained slowness.
- **Spikes in category percentages**: a sudden shift in the category bar
	 (e.g., NET or FS) often points to the subsystem responsible for latency or
	 errors.

Quick API queries for triage:

```bash
curl localhost:8080/api/stats | jq .            # snapshot of all syscalls
curl localhost:8080/api/syscall/openat | jq .      # detail + error breakdown
curl localhost:8080/api/log | jq .                 # last 500 raw events
```

Common patterns and notes:

- `openat` with many `ENOENT` entries — often the dynamic linker probing
	 for shared libraries; usually informational.
- `connect` with ~50% failures — may be Happy Eyeballs (parallel IPv4/IPv6)
	 behaviour; not necessarily a problem.
- `ioctl` 100% errors — typically a missing TTY or unsupported operation; the
	 detail overlay explains common causes.

When in doubt, enable `--debug` (local troubleshooting only) and collect an
HTML report with `--report` to share a self-contained snapshot of the
session.

See also: [HTTP API]({{< relref "docs/api.md" >}}) and the [Usage Guide]({{< relref "docs/usage.md" >}}).

## Categories at a glance

| Category | Label | Syscalls included |
| ---------- | ------- | ------------------- |
| File descriptor I/O | **I/O** | `read`, `write`, `openat`, `close`, `pipe`, `dup`, `sendfile`, `fcntl` |
| Filesystem metadata | **FS** | `fstat`, `getdents64`, `access`, `lseek`, `statfs` |
| Networking & sockets | **NET** | `socket`, `bind`, `listen`, `connect`, `accept4`, `recvfrom`, `sendto`, `setsockopt`, `getsockname`, `epoll_wait`, `epoll_ctl`, `poll` |
| Memory management | **MEM** | `mmap`, `munmap`, `mprotect`, `madvise`, `brk` |
| Process control | **PROC** | `clone`, `execve`, `exit_group`, `wait4`, `getpid`, `getuid`, `prctl`, `prlimit64`, `set_tid_address`, `arch_prctl` |
| Signal handling | **SIG** | `rt_sigaction`, `rt_sigprocmask`, `eventfd` |
| Other | **OTHER** | `futex`, `ioctl`, `getrandom` |

---

## I/O — File descriptor read / write / open / close

### `read`

Read bytes from a file descriptor into a buffer.

**Signature:** `read(fd, buf, count) → bytes_read`

| Argument | Description |
| ---------- | ----------- |
| `fd` | open file descriptor to read from |
| `buf` | destination buffer in user space |
| `count` | maximum number of bytes to read |

**Returns:** number of bytes read; `0` = EOF  
**Common errors:** `EAGAIN` (non-blocking, no data), `EBADF` (bad fd), `EFAULT` (bad buffer), `EINTR` (interrupted by signal)  
**Notes:** High `read()` counts often indicate heavy file or socket data transfer. `EAGAIN` is expected for non-blocking I/O and is not a real error.

---

### `write`

Write bytes from a buffer to a file descriptor.

**Signature:** `write(fd, buf, count) → bytes_written`

| Argument | Description |
| ---------- | ----------- |
| `fd` | open file descriptor to write to |
| `buf` | source buffer in user space |
| `count` | number of bytes to write |

**Returns:** number of bytes written  
**Common errors:** `EAGAIN` (non-blocking, buffer full), `EBADF`, `EPIPE` (peer closed), `EFAULT`  
**Notes:** A short write (return < `count`) can happen on sockets or pipes; callers should loop.

---

### `openat`

Open or create a file, returning a file descriptor.

**Signature:** `openat(dirfd, pathname, flags, mode) → fd`  
**Aliases:** `open`

| Argument | Description |
| ---------- | ----------- |
| `dirfd` | `AT_FDCWD` or directory fd for relative paths |
| `pathname` | path to file |
| `flags` | `O_RDONLY`, `O_WRONLY`, `O_CREAT`, `O_TRUNC`, … |
| `mode` | permission bits when `O_CREAT` is used |

**Returns:** new file descriptor (≥ 0)  
**Common errors:** `ENOENT` (not found), `EACCES` (permission denied), `EMFILE` (too many open fds), `EEXIST` (`O_CREAT|O_EXCL`)  
**Notes:** High `ENOENT` error rates are normal — the dynamic linker probes many paths when loading shared libraries.

---

### `close`

Close a file descriptor, releasing the kernel resource.

**Signature:** `close(fd) → 0`

| Argument | Description |
| ---------- | ----------- |
| `fd` | file descriptor to close |

**Returns:** `0`  
**Common errors:** `EBADF` (fd not open), `EIO` (deferred write-back failed — data may be lost)  
**Notes:** Never ignore `EIO` on `close()` — it means buffered data was not written to disk.

---

### `pipe`

Create a unidirectional data channel between two file descriptors.

**Signature:** `pipe2(pipefd[2], flags) → 0`  
**Aliases:** `pipe2`

| Argument | Description |
| ---------- | ----------- |
| `pipefd` | `[0]` = read end, `[1]` = write end |
| `flags` | `O_CLOEXEC`, `O_NONBLOCK`, `O_DIRECT` |

**Returns:** `0`

---

### `dup`

Duplicate a file descriptor.

**Signature:** `dup2(oldfd, newfd) → newfd`  
**Aliases:** `dup2`, `dup3`

| Argument | Description |
| ---------- | ----------- |
| `oldfd` | fd to duplicate |
| `newfd` | desired fd number (closed first if already open) |

**Returns:** new file descriptor

---

### `sendfile`

Transfer data between two file descriptors entirely in kernel space (zero-copy).

**Signature:** `sendfile(out_fd, in_fd, offset, count) → bytes_sent`  
**Aliases:** `copy_file_range`

**Notes:** Data never crosses user space. Used by web servers to send file contents over sockets with minimal overhead.

---

### `fcntl`

Perform miscellaneous operations on a file descriptor (flags, locks, async I/O).

**Signature:** `fcntl(fd, cmd, arg) → value`

| Argument | Description |
| ---------- | ------------- |
| `cmd` | `F_GETFL`, `F_SETFL` (`O_NONBLOCK`), `F_GETFD`, `F_SETFD` (`FD_CLOEXEC`), `F_DUPFD`, `F_SETLK`, … |

---

## FS — Filesystem metadata

### `fstat`

Retrieve file metadata (size, permissions, timestamps, inode).

**Signature:** `fstat(fd, statbuf) → 0` / `stat(pathname, statbuf) → 0`  
**Aliases:** `stat`, `lstat`, `newfstatat`, `statx`

| Argument | Description |
| ---------- | ----------- |
| `fd` / `pathname` | file descriptor or path to inspect |
| `statbuf` | `struct stat` to fill in |

**Returns:** `0`  
**Common errors:** `ENOENT` (path not found), `EACCES`, `EBADF`  
**Notes:** High `fstat`/`stat` rates are normal when an application polls file state or a web server serves many files.

---

### `getdents64`

Read directory entries from an open directory file descriptor.

**Signature:** `getdents64(fd, dirp, count) → bytes_read`  
**Aliases:** `getdents`

| Argument | Description |
| ---------- | ----------- |
| `fd` | directory fd |
| `dirp` | buffer for `linux_dirent64` structs |
| `count` | buffer size |

**Returns:** bytes read; `0` = end of directory  
**Common errors:** `EBADF`, `ENOTDIR` (fd is not a directory)  
**Notes:** Used by `readdir(3)`. High counts suggest directory scanning (file watchers, recursive search, `find`).

---

### `access`

Check whether the calling process can access a file.

**Signature:** `access(pathname, mode) → 0`  
**Aliases:** `faccessat`, `faccessat2`

| Argument | Description |
| ---------- | ------------- |
| `pathname` | path to check |
| `mode` | `F_OK` (exists?), `R_OK`, `W_OK`, `X_OK` |

**Returns:** `0` if access is allowed  
**Common errors:** `ENOENT` (not found — usually harmless), `EACCES` (permission denied)  
**Notes:** High `ENOENT` rates are expected — programs routinely probe for optional config files. Not a real error.

---

### `lseek`

Reposition the read/write offset of a file descriptor.

**Signature:** `lseek(fd, offset, whence) → new_offset`  
**Aliases:** `llseek`

| Argument | Description |
| ---------- | ------------- |
| `whence` | `SEEK_SET` (absolute), `SEEK_CUR` (relative), `SEEK_END` (from end) |

**Returns:** resulting file offset  
**Common errors:** `ESPIPE` (fd is a pipe or socket — not seekable), `EINVAL`

---

### `statfs`

Get filesystem statistics (type, free space, block size, inode counts).

**Signature:** `statfs(pathname, buf) → 0`  
**Aliases:** `fstatfs`

| Argument | Description |
| ---------- | ----------- |
| `pathname` | path on the filesystem to inspect |
| `buf` | `struct statfs` to fill |

**Returns:** `0`  
**Common errors:** `ENOENT`, `EACCES`, `ENOSYS` (on special filesystems like `/proc`)  
**Notes:** Errors on `/proc` or `/sys` are expected — those filesystems may not support `statfs`.

---

## NET — Networking & sockets

### `socket`

Create a communication endpoint (socket).

**Signature:** `socket(domain, type, protocol) → fd`

| Argument | Description |
| ---------- | ----------- |
| `domain` | `AF_INET`, `AF_INET6`, `AF_UNIX`, `AF_NETLINK`, … |
| `type` | `SOCK_STREAM`, `SOCK_DGRAM`, `SOCK_RAW` \| `SOCK_NONBLOCK` \| `SOCK_CLOEXEC` |
| `protocol` | `0` (auto), `IPPROTO_TCP`, `IPPROTO_UDP`, … |

**Returns:** new socket fd

---

### `bind`

Assign a local address to a socket.

**Signature:** `bind(sockfd, addr, addrlen) → 0`

| Argument | Description |
| ---------- | ----------- |
| `addr` | local address (port + IP, or Unix socket path) |

**Returns:** `0`  
**Common errors:** `EADDRINUSE` (port already in use), `EACCES` (port < 1024 without `CAP_NET_BIND_SERVICE`)

---

### `listen`

Mark a socket as passive, ready to accept connections.

**Signature:** `listen(sockfd, backlog) → 0`

| Argument | Description |
| ---------- | ----------- |
| `backlog` | max length of the pending connection queue |

**Returns:** `0`

---

### `connect`

Initiate a connection on a socket.

**Signature:** `connect(sockfd, addr, addrlen) → 0`

| Argument | Description |
| ---------- | ----------- |
| `sockfd` | open socket fd |
| `addr` | target address (`sockaddr_in`, `sockaddr_un`, …) |
| `addrlen` | `sizeof(*addr)` |

**Returns:** `0` on success  
**Common errors:** `ECONNREFUSED` (port closed), `ETIMEDOUT`, `ENETUNREACH`, `EINPROGRESS` (non-blocking)  
**Notes:** Errors are common with Happy Eyeballs (RFC 8305) — both IPv4 and IPv6 are tried in parallel; the loser always fails with `ECONNREFUSED` or `ETIMEDOUT`. This is expected behaviour, not a real failure.

---

### `accept4`

Accept a new incoming connection on a listening socket.

**Signature:** `accept4(sockfd, addr, addrlen, flags) → fd`  
**Aliases:** `accept`

| Argument | Description |
| ---------- | ----------- |
| `sockfd` | listening socket fd |
| `addr` | filled with the peer address |
| `flags` | `SOCK_NONBLOCK`, `SOCK_CLOEXEC` |

**Returns:** new connected socket fd  
**Common errors:** `EAGAIN` (no pending connections, non-blocking), `EMFILE` (too many open fds)

---

### `recvfrom`

Receive data from a socket.

**Signature:** `recvfrom(sockfd, buf, len, flags, src_addr, addrlen) → bytes`  
**Aliases:** `recv`, `recvmsg`, `recvmmsg`

| Argument | Description |
| ---------- | ----------- |
| `sockfd` | connected or unconnected socket |
| `buf` | receive buffer |
| `flags` | `MSG_DONTWAIT`, `MSG_PEEK`, `MSG_WAITALL`, … |

**Returns:** bytes received; `0` = peer closed  
**Common errors:** `EAGAIN`/`EWOULDBLOCK` (non-blocking, no data yet — normal), `ECONNRESET`  
**Notes:** `EAGAIN` on a non-blocking socket is not a real error — the event loop will retry when data arrives.

---

### `sendto`

Send data through a socket.

**Signature:** `sendto(sockfd, buf, len, flags, dest_addr, addrlen) → bytes`  
**Aliases:** `send`, `sendmsg`, `sendmmsg`

| Argument | Description |
| ---------- | ----------- |
| `sockfd` | socket fd |
| `buf` | data to send |
| `flags` | `MSG_DONTWAIT`, `MSG_NOSIGNAL`, … |

**Returns:** bytes sent  
**Common errors:** `EPIPE` (peer closed — usually triggers `SIGPIPE` too), `EAGAIN`, `ECONNRESET`

---

### `setsockopt`

Set or get socket options (timeouts, buffers, `TCP_NODELAY`, `SO_REUSEADDR`, …).

**Signature:** `setsockopt(sockfd, level, optname, optval, optlen) → 0`  
**Aliases:** `getsockopt`

| Argument | Description |
| ---------- | ----------- |
| `level` | `SOL_SOCKET`, `IPPROTO_TCP`, `IPPROTO_IP`, … |
| `optname` | `SO_REUSEADDR`, `SO_KEEPALIVE`, `TCP_NODELAY`, `SO_RCVBUF`, … |

---

### `getsockname`

Get the local (`getsockname`) or remote (`getpeername`) address of a socket.

**Signature:** `getsockname(sockfd, addr, addrlen) → 0`  
**Aliases:** `getpeername`

---

### `epoll_wait`

Wait for I/O events on an epoll file descriptor.

---

## Event format

`stracectl` represents each captured syscall as a `SyscallEvent`. When
exposed via the HTTP API or WebSocket stream this is typically serialized as
JSON with the following fields:

- `pid` — process id of the caller
- `name` — canonical syscall name (e.g. `openat`, `read`)
- `args` — stringified arguments as presented by the tracer/parser
- `ret` / `retval` — return value from the syscall (string)
- `latency_ns` — kernel time spent in the syscall (nanoseconds)
- `error` — POSIX errno name when the call failed (e.g. `ENOENT`)
- `timestamp` — ISO8601 timestamp when the event was recorded

Example (API / WebSocket payload):

```json
{
	"pid": 1234,
	"name": "openat",
	"args": "AT_FDCWD, \"/etc/ld.so.conf\", O_RDONLY",
	"ret": "3",
	"latency_ns": 38200,
	"error": "",
	"timestamp": "2026-03-09T12:34:56.789Z"
}
```

Internally the Go type is `models.SyscallEvent` with fields `PID`, `Name`,
`Args`, `RetVal`, `Error`, `Latency` (time.Duration) and `Time` (time.Time).

**Signature:** `epoll_wait(epfd, events, maxevents, timeout) → n_events`  
**Aliases:** `epoll_pwait`

| Argument | Description |
| ---------- | ----------- |
| `epfd` | epoll instance fd |
| `events` | array of `epoll_event` structs to fill |
| `maxevents` | max events to return per call |
| `timeout` | milliseconds to wait (`-1` = block indefinitely) |

**Returns:** number of ready fds; `0` = timeout  
**Notes:** The main blocking call in event-driven servers (nginx, Node.js, Go net poller). A high call count means many I/O events are being processed.

---

### `epoll_ctl`

Add, modify, or remove a file descriptor from an epoll instance.

**Signature:** `epoll_ctl(epfd, op, fd, event) → 0`

| Argument | Description |
| ---------- | ----------- |
| `op` | `EPOLL_CTL_ADD`, `EPOLL_CTL_MOD`, `EPOLL_CTL_DEL` |
| `fd` | target file descriptor |
| `event` | `epoll_event` with events mask and user data |

**Returns:** `0`  
**Common errors:** `ENOENT` (`DEL`/`MOD` on fd not registered), `EEXIST` (`ADD` on already registered fd)

---

### `poll`

Wait for events on a set of file descriptors.

**Signature:** `poll(fds, nfds, timeout) → n_ready`  
**Aliases:** `ppoll`

| Argument | Description |
| ---------- | ----------- |
| `fds` | array of `pollfd` structs |
| `nfds` | number of fds to monitor |
| `timeout` | milliseconds (`-1` = block indefinitely) |

**Returns:** number of fds with events; `0` = timeout; `-1` = error

---

## MEM — Memory management

### `mmap`

Map files or devices into memory, or allocate anonymous memory.

**Signature:** `mmap(addr, length, prot, flags, fd, offset) → addr`  
**Aliases:** `mmap2`

| Argument | Description |
| ---------- | ----------- |
| `addr` | hint for mapping address (`0` = kernel decides) |
| `length` | size in bytes |
| `prot` | `PROT_READ` \| `PROT_WRITE` \| `PROT_EXEC` |
| `flags` | `MAP_PRIVATE`, `MAP_SHARED`, `MAP_ANONYMOUS`, … |
| `fd` | file to map, or `-1` for anonymous |
| `offset` | file offset (must be page-aligned) |

**Returns:** virtual address of the mapping  
**Common errors:** `ENOMEM` (out of virtual address space), `EACCES`, `EINVAL`  
**Notes:** `MAP_ANONYMOUS|MAP_PRIVATE` is how `malloc` allocates large blocks from the kernel.

---

### `munmap`

Remove a memory mapping created by `mmap`.

**Signature:** `munmap(addr, length) → 0`

| Argument | Description |
| ---------- | ----------- |
| `addr` | start of the mapping (must be page-aligned) |
| `length` | size to unmap |

**Returns:** `0`  
**Common errors:** `EINVAL` (addr not aligned or not mapped)

---

### `mprotect`

Change memory protection attributes on a mapped region.

**Signature:** `mprotect(addr, len, prot) → 0`

| Argument | Description |
| ---------- | ----------- |
| `addr` | page-aligned start address |
| `len` | length in bytes |
| `prot` | `PROT_NONE` \| `PROT_READ` \| `PROT_WRITE` \| `PROT_EXEC` |

**Returns:** `0`  
**Common errors:** `EACCES` (e.g. making a file-backed mapping writable without write permission), `EINVAL`  
**Notes:** Frequently called by the dynamic linker (`ld.so`) during library loading — setting sections read-only after relocation is complete.

---

### `madvise`

Give the kernel hints on expected memory usage patterns.

**Signature:** `madvise(addr, length, advice) → 0`

| Argument | Description |
| ---------- | ----------- |
| `addr` | page-aligned start |
| `length` | region size |
| `advice` | `MADV_NORMAL`, `MADV_SEQUENTIAL`, `MADV_DONTNEED`, `MADV_FREE`, … |

**Returns:** `0`  
**Common errors:** `EINVAL` (unknown advice or bad addr/length), `EACCES`  
**Notes:** Errors from `madvise` are informational — the kernel ignores hints it cannot honour. Not a real failure.

---

### `brk`

Adjust the end of the data segment (heap boundary).

**Signature:** `brk(addr) → new_brk`

| Argument | Description |
| ---------- | ----------- |
| `addr` | new end of heap (`0` = query current value) |

**Returns:** current break address  
**Notes:** Modern `malloc` implementations prefer `mmap` for large allocations. `brk` is used for the initial small heap.

---

## PROC — Process control

### `clone`

Create a new process or thread with fine-grained control over shared resources.

**Signature:** `clone(flags, stack, ptid, ctid, regs) → child_pid`  
**Aliases:** `clone3`

| Argument | Description |
| ---------- | ----------- |
| `flags` | `CLONE_THREAD`, `CLONE_VM`, `CLONE_FS`, `SIGCHLD`, … |
| `stack` | new stack pointer for the child (`0` = copy parent stack) |

**Returns:** child PID in parent; `0` in child  
**Notes:** `pthread_create(3)` uses `clone` with `CLONE_THREAD|CLONE_VM`. `fork(2)` uses `clone` with `SIGCHLD` only.

---

### `execve`

Replace the current process image with a new program.

**Signature:** `execve(pathname, argv, envp)` → does not return on success  
**Aliases:** `execveat`

| Argument | Description |
| ---------- | ----------- |
| `pathname` | path to the executable |
| `argv` | argument vector (NULL-terminated array of strings) |
| `envp` | environment strings |

**Returns:** does not return; `-1` on error  
**Common errors:** `ENOENT` (not found), `EACCES` (not executable), `ENOEXEC` (bad ELF), `ENOMEM`

---

### `exit_group`

Terminate the calling thread (`exit`) or all threads in the thread group (`exit_group`).

**Signature:** `exit_group(status)` → does not return  
**Aliases:** `exit`

| Argument | Description |
| ---------- | ----------- |
| `status` | exit code (low 8 bits visible to `waitpid`) |

**Notes:** `exit_group` is what `libc exit(3)` calls. It ensures all threads in the process terminate cleanly.

---

### `wait4`

Wait for a child process to change state.

**Signature:** `wait4(pid, status, options, rusage) → child_pid`  
**Aliases:** `waitpid`, `waitid`

| Argument | Description |
| ---------- | ----------- |
| `pid` | `-1` = any child; `>0` = specific PID |
| `options` | `WNOHANG` (non-blocking), `WUNTRACED`, `WCONTINUED` |

**Returns:** PID of the child that changed state; `0` with `WNOHANG` if none ready

---

### `getpid`

Return the process ID of the calling process.

**Signature:** `getpid() → pid`

**Notes:** Modern Linux caches the PID in the vDSO — this syscall may never actually enter the kernel.

---

### `getuid`

Return the real/effective user or group ID of the calling process.

**Signature:** `getuid() → uid`  
**Aliases:** `geteuid`, `getgid`, `getegid`

**Notes:** Very cheap; usually cached by libc. Frequent calls suggest credential-checking code paths.

---

### `prctl`

Control various process attributes (name, seccomp, capabilities, …).

**Signature:** `prctl(option, arg2, arg3, arg4, arg5) → 0 or value`

| Argument | Description |
| ---------- | ----------- |
| `option` | `PR_SET_NAME`, `PR_SET_SECCOMP`, `PR_CAP_AMBIENT`, `PR_SET_DUMPABLE`, … |

**Returns:** `0` or option-specific value  
**Common errors:** `EPERM` (capability required), `EINVAL` (unknown option)  
**Notes:** `EPERM` on `prctl` is common in containers with restricted capabilities or active seccomp profiles.

---

### `prlimit64`

Get or set resource limits (CPU time, memory, open files, …) for a process.

**Signature:** `prlimit64(pid, resource, new_limit, old_limit) → 0`

| Argument | Description |
| ---------- | ----------- |
| `resource` | `RLIMIT_NOFILE`, `RLIMIT_AS`, `RLIMIT_STACK`, `RLIMIT_CORE`, … |
| `pid` | `0` = calling process |

---

### `set_tid_address`

Set the address that the kernel will clear when the thread exits.

**Signature:** `set_tid_address(tidptr) → tid`

**Notes:** Called once at thread startup by `glibc`. Used for robust futex cleanup on thread exit. Normal to see once per thread.

---

### `arch_prctl`

Set architecture-specific thread state (e.g. FS/GS segment base for TLS).

**Signature:** `arch_prctl(code, addr) → 0`

| Argument | Description |
| ---------- | ----------- |
| `code` | `ARCH_SET_FS` (set FS base for thread-local storage), `ARCH_GET_FS`, … |

**Notes:** Called once per thread by `glibc` to initialise thread-local storage (TLS). Normal during process startup.

---

## SIG — Signal handling

### `rt_sigaction`

Install or query a signal handler.

**Signature:** `rt_sigaction(signum, act, oldact, sigsetsize) → 0`  
**Aliases:** `sigaction`

| Argument | Description |
| ---------- | ----------- |
| `signum` | signal number (`SIGINT`, `SIGSEGV`, …) |
| `act` | new `sigaction` struct (`NULL` = query only) |
| `oldact` | previous handler (`NULL` = discard) |

**Returns:** `0`

---

### `rt_sigprocmask`

Block, unblock, or query the set of blocked signals.

**Signature:** `rt_sigprocmask(how, set, oldset, sigsetsize) → 0`  
**Aliases:** `sigprocmask`

| Argument | Description |
| ---------- | ----------- |
| `how` | `SIG_BLOCK`, `SIG_UNBLOCK`, `SIG_SETMASK` |
| `set` | new signal mask (`NULL` = query only) |
| `oldset` | previous mask |

**Notes:** Called very frequently by Go and pthreads runtimes around goroutine/thread switches.

---

### `eventfd`

Create a file descriptor for event notification between threads or processes.

**Signature:** `eventfd2(initval, flags) → fd`  
**Aliases:** `eventfd2`

| Argument | Description |
| ---------- | ----------- |
| `initval` | initial counter value |
| `flags` | `EFD_NONBLOCK`, `EFD_CLOEXEC`, `EFD_SEMAPHORE` |

**Notes:** Used by the Go runtime and libuv/libevent to wake up blocked pollers without a pipe. More efficient than a pipe pair.

---

## OTHER

### `futex`

Fast user-space locking primitive — the kernel backing for mutexes and condition variables.

**Signature:** `futex(uaddr, op, val, timeout, uaddr2, val3) → 0 or value`

| Argument | Description |
| ---------- | ----------- |
| `uaddr` | address of the futex word (shared between threads) |
| `op` | `FUTEX_WAIT`, `FUTEX_WAKE`, `FUTEX_LOCK_PI`, … |
| `val` | expected value (for `WAIT`) or wake count (for `WAKE`) |

**Notes:** Most of the time `futex` stays in user space with no syscall. A syscall happens only when a thread must actually sleep or be woken. High syscall counts suggest heavy lock contention.

---

### `ioctl`

Device-specific control operations on a file descriptor.

**Signature:** `ioctl(fd, request, argp) → 0 or value`

| Argument | Description |
| ---------- | ----------- |
| `fd` | open file descriptor (device, socket, terminal, …) |
| `request` | device-specific command code (`TIOCGWINSZ`, `FIONREAD`, …) |
| `argp` | pointer to in/out argument |

**Returns:** `0` or a request-specific value  
**Common errors:** `ENOTTY` (fd is not a terminal — very common when stdout is piped), `EINVAL`, `ENODEV`  
**Notes:** `ENOTTY` is expected when the process checks for a TTY but is running under `sudo`, in a container, or with piped output. Not a real failure.

---

### `getrandom`

Obtain cryptographically secure random bytes from the kernel entropy pool.

**Signature:** `getrandom(buf, buflen, flags) → bytes_filled`

| Argument | Description |
| ---------- | ----------- |
| `flags` | `0` (block until entropy ready), `GRND_NONBLOCK`, `GRND_RANDOM` |

**Returns:** number of bytes filled  
**Notes:** The preferred alternative to `/dev/urandom`. Called at startup by TLS libraries and language runtimes (Go, Python, Node.js) for seed material.

---

## Alias table

The following names are automatically normalised to their canonical entry.

| Alias | Canonical |
| ------- | ----------- |
| `open` | `openat` |
| `mmap2` | `mmap` |
| `stat`, `lstat`, `newfstatat`, `statx` | `fstat` |
| `getdents` | `getdents64` |
| `faccessat`, `faccessat2` | `access` |
| `accept` | `accept4` |
| `recv`, `recvmsg`, `recvmmsg` | `recvfrom` |
| `send`, `sendmsg`, `sendmmsg` | `sendto` |
| `epoll_pwait` | `epoll_wait` |
| `ppoll` | `poll` |
| `clone3` | `clone` |
| `execveat` | `execve` |
| `exit` | `exit_group` |
| `waitpid`, `waitid` | `wait4` |
| `sigaction` | `rt_sigaction` |
| `sigprocmask` | `rt_sigprocmask` |
| `geteuid`, `getgid`, `getegid` | `getuid` |
| `llseek` | `lseek` |
| `pipe2` | `pipe` |
| `dup2`, `dup3` | `dup` |
| `getsockopt` | `setsockopt` |
| `getpeername` | `getsockname` |
| `fstatfs` | `statfs` |
| `copy_file_range` | `sendfile` |
| `eventfd2` | `eventfd` |
