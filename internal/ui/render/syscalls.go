package render

import (
	"fmt"
)

// SyscallDetail holds reference information for one syscall.
type SyscallDetail struct {
	Description string
	Signature   string
	Args        [][2]string // [name, description]
	ReturnValue string
	ErrorHint   string
	Notes       string
}

// SyscallAliases maps variant/legacy syscall names to their canonical key in SyscallDetails.
var SyscallAliases = map[string]string{
	"open":            "openat",
	"mmap2":           "mmap",
	"stat":            "fstat",
	"lstat":           "fstat",
	"newfstatat":      "fstat",
	"statx":           "fstat",
	"getdents":        "getdents64",
	"faccessat":       "access",
	"faccessat2":      "access",
	"accept":          "accept4",
	"recv":            "recvfrom",
	"recvmsg":         "recvfrom",
	"recvmmsg":        "recvfrom",
	"send":            "sendto",
	"sendmsg":         "sendto",
	"sendmmsg":        "sendto",
	"epoll_pwait":     "epoll_wait",
	"ppoll":           "poll",
	"clone3":          "clone",
	"execveat":        "execve",
	"exit":            "exit_group",
	"waitpid":         "wait4",
	"waitid":          "wait4",
	"sigaction":       "rt_sigaction",
	"sigprocmask":     "rt_sigprocmask",
	"geteuid":         "getuid",
	"getgid":          "getuid",
	"getegid":         "getuid",
	"llseek":          "lseek",
	"pipe2":           "pipe",
	"dup2":            "dup",
	"dup3":            "dup",
	"getsockopt":      "setsockopt",
	"getpeername":     "getsockname",
	"fstatfs":         "statfs",
	"copy_file_range": "sendfile",
	"eventfd2":        "eventfd",
}

// SyscallDetails maps canonical syscall names to human-readable reference data.
var SyscallDetails = map[string]SyscallDetail{
	"read": {
		Description: "Read bytes from a file descriptor into a buffer.",
		Signature:   "read(fd, buf, count) → bytes_read",
		Args:        [][2]string{{"fd", "open file descriptor to read from"}, {"buf", "destination buffer in user space"}, {"count", "maximum number of bytes to read"}},
		ReturnValue: "number of bytes read (0 = EOF)",
		ErrorHint:   "EAGAIN (non-blocking, no data), EBADF (bad fd), EFAULT (bad buffer), EINTR (signal)",
		Notes:       "High read() counts often indicate heavy file or socket data transfer. EAGAIN is expected for non-blocking I/O and not a real error.",
	},
	"write": {
		Description: "Write bytes from a buffer to a file descriptor.",
		Signature:   "write(fd, buf, count) → bytes_written",
		Args:        [][2]string{{"fd", "open file descriptor to write to"}, {"buf", "source buffer in user space"}, {"count", "number of bytes to write"}},
		ReturnValue: "number of bytes written",
		ErrorHint:   "EAGAIN (non-blocking, buffer full), EBADF, EPIPE (peer closed), EFAULT",
		Notes:       "A short write (return < count) can happen on sockets or pipes; callers should loop.",
	},
	"openat": {
		Description: "Open or create a file, returning a file descriptor.",
		Signature:   "openat(dirfd, pathname, flags, mode) → fd",
		Args:        [][2]string{{"dirfd", "AT_FDCWD or directory fd for relative path"}, {"pathname", "path to file"}, {"flags", "O_RDONLY, O_WRONLY, O_CREAT, O_TRUNC, …"}, {"mode", "permission bits when O_CREAT is used"}},
		ReturnValue: "new file descriptor (≥ 0)",
		ErrorHint:   "ENOENT (not found), EACCES (permission), EMFILE (too many open fds), EEXIST (O_CREAT|O_EXCL)",
		Notes:       "High ENOENT error rates are normal: the dynamic linker probes many paths when loading shared libraries.",
	},
	"close": {
		Description: "Close a file descriptor, releasing the kernel resource.",
		Signature:   "close(fd) → 0",
		Args:        [][2]string{{"fd", "file descriptor to close"}},
		ReturnValue: "0",
		ErrorHint:   "EBADF (fd not open), EIO (deferred write-back failed — data may be lost)",
		Notes:       "Never ignore EIO on close() — it means data was not written to disk.",
	},
	"mmap": {
		Description: "Map files or devices into memory, or allocate anonymous memory.",
		Signature:   "mmap(addr, length, prot, flags, fd, offset) → addr",
		Args:        [][2]string{{"addr", "hint for mapping address (usually 0 = kernel decides)"}, {"length", "size in bytes"}, {"prot", "PROT_READ | PROT_WRITE | PROT_EXEC"}, {"flags", "MAP_PRIVATE, MAP_SHARED, MAP_ANONYMOUS, …"}, {"fd", "file to map, or -1 for anonymous"}, {"offset", "file offset (must be page-aligned)"}},
		ReturnValue: "virtual address of the mapping (hex)",
		ErrorHint:   "ENOMEM (out of virtual address space), EACCES, EINVAL",
		Notes:       "MAP_ANONYMOUS|MAP_PRIVATE is the usual way malloc allocates large blocks from the kernel.",
	},
	"munmap": {
		Description: "Remove a memory mapping created by mmap.",
		Signature:   "munmap(addr, length) → 0",
		Args:        [][2]string{{"addr", "start of the mapping (must be page-aligned)"}, {"length", "size to unmap"}},
		ReturnValue: "0",
		ErrorHint:   "EINVAL (addr not aligned or not mapped)",
	},
	"mprotect": {
		Description: "Change memory protection attributes on a mapped region.",
		Signature:   "mprotect(addr, len, prot) → 0",
		Args:        [][2]string{{"addr", "page-aligned start address"}, {"len", "length in bytes"}, {"prot", "PROT_NONE | PROT_READ | PROT_WRITE | PROT_EXEC"}},
		ReturnValue: "0",
		ErrorHint:   "EACCES (e.g. trying to make a file-backed mapping writable without write permission), EINVAL",
		Notes:       "Frequently called by the dynamic linker (ld.so) during library loading — setting sections read-only after relocation.",
	},
	"madvise": {
		Description: "Give the kernel hints on expected memory usage patterns.",
		Signature:   "madvise(addr, length, advice) → 0",
		Args:        [][2]string{{"addr", "page-aligned start"}, {"length", "region size"}, {"advice", "MADV_NORMAL, MADV_SEQUENTIAL, MADV_DONTNEED, MADV_FREE, …"}},
		ReturnValue: "0",
		ErrorHint:   "EINVAL (unknown advice or bad addr/length), EACCES",
		Notes:       "EACCES or EINVAL errors are informational — the kernel ignores hints it cannot honour. Not a real failure.",
	},
	"brk": {
		Description: "Adjust the end of the data segment (heap boundary).",
		Signature:   "brk(addr) → new_brk",
		Args:        [][2]string{{"addr", "new end of heap (0 = query current value)"}},
		ReturnValue: "current break address",
		Notes:       "Modern malloc implementations prefer mmap for large allocations; brk is used for the initial heap.",
	},
	"fstat": {
		Description: "Retrieve file metadata (size, permissions, timestamps, inode).",
		Signature:   "fstat(fd, statbuf) → 0  /  stat(pathname, statbuf) → 0",
		Args:        [][2]string{{"fd / pathname", "file descriptor or path to inspect"}, {"statbuf", "struct stat to fill in"}},
		ReturnValue: "0",
		ErrorHint:   "ENOENT (path not found), EACCES, EBADF",
		Notes:       "High fstat/stat rates are normal when an application polls file state or a web server serves many files.",
	},
	"getdents64": {
		Description: "Read directory entries from an open directory file descriptor.",
		Signature:   "getdents64(fd, dirp, count) → bytes_read",
		Args:        [][2]string{{"fd", "directory fd"}, {"dirp", "buffer for linux_dirent64 structs"}, {"count", "buffer size"}},
		ReturnValue: "bytes read (0 = end of directory)",
		ErrorHint:   "EBADF, ENOTDIR (fd is not a directory)",
		Notes:       "Used by readdir(3). High counts suggest directory scanning (e.g. file watchers, recursive search).",
	},
	"access": {
		Description: "Check whether the calling process can access a file.",
		Signature:   "access(pathname, mode) → 0",
		Args:        [][2]string{{"pathname", "path to check"}, {"mode", "F_OK (exists?), R_OK, W_OK, X_OK"}},
		ReturnValue: "0 if access allowed",
		ErrorHint:   "ENOENT (not found — usually harmless), EACCES (permission denied)",
		Notes:       "High ENOENT rates are expected: programs probe for optional config files. Not a real error.",
	},
	"connect": {
		Description: "Initiate a connection on a socket.",
		Signature:   "connect(sockfd, addr, addrlen) → 0",
		Args:        [][2]string{{"sockfd", "open socket fd"}, {"addr", "target address (sockaddr_in, sockaddr_un, …)"}, {"addrlen", "sizeof(*addr)"}},
		ReturnValue: "0 on success",
		ErrorHint:   "ECONNREFUSED (port closed), ETIMEDOUT, ENETUNREACH, EINPROGRESS (non-blocking)",
		Notes:       "Errors are common with Happy Eyeballs (RFC 8305): both IPv4 and IPv6 are tried in parallel; the loser always fails with ECONNREFUSED or ETIMEDOUT.",
	},
	"accept4": {
		Description: "Accept a new incoming connection on a listening socket.",
		Signature:   "accept4(sockfd, addr, addrlen, flags) → fd",
		Args:        [][2]string{{"sockfd", "listening socket fd"}, {"addr", "filled with peer address"}, {"flags", "SOCK_NONBLOCK, SOCK_CLOEXEC"}},
		ReturnValue: "new connected socket fd",
		ErrorHint:   "EAGAIN (no pending connections, non-blocking), EMFILE (too many open fds)",
	},
	"recvfrom": {
		Description: "Receive data from a socket.",
		Signature:   "recvfrom(sockfd, buf, len, flags, src_addr, addrlen) → bytes",
		Args:        [][2]string{{"sockfd", "connected or unconnected socket"}, {"buf", "receive buffer"}, {"flags", "MSG_DONTWAIT, MSG_PEEK, MSG_WAITALL, …"}},
		ReturnValue: "bytes received (0 = peer closed)",
		ErrorHint:   "EAGAIN/EWOULDBLOCK (non-blocking, no data yet — normal), ECONNRESET",
		Notes:       "EAGAIN on a non-blocking socket is not a real error — the event loop will retry.",
	},
	"sendto": {
		Description: "Send data through a socket.",
		Signature:   "sendto(sockfd, buf, len, flags, dest_addr, addrlen) → bytes",
		Args:        [][2]string{{"sockfd", "socket fd"}, {"buf", "data to send"}, {"flags", "MSG_DONTWAIT, MSG_NOSIGNAL, …"}},
		ReturnValue: "bytes sent",
		ErrorHint:   "EPIPE (peer closed — usually triggers SIGPIPE too), EAGAIN, ECONNRESET",
	},
	"epoll_wait": {
		Description: "Wait for events on an epoll file descriptor.",
		Signature:   "epoll_wait(epfd, events, maxevents, timeout) → n_events",
		Args:        [][2]string{{"epfd", "epoll instance fd"}, {"events", "array of epoll_event to fill"}, {"maxevents", "max events to return"}, {"timeout", "ms to wait (-1 = block forever)"}},
		ReturnValue: "number of ready fds (0 = timeout)",
		Notes:       "The main blocking call in event-driven servers (nginx, Node.js, Go net poller). High count = many I/O events.",
	},
	"epoll_ctl": {
		Description: "Add, modify, or remove a file descriptor from an epoll instance.",
		Signature:   "epoll_ctl(epfd, op, fd, event) → 0",
		Args:        [][2]string{{"op", "EPOLL_CTL_ADD, EPOLL_CTL_MOD, EPOLL_CTL_DEL"}, {"fd", "target file descriptor"}, {"event", "epoll_event with events mask and user data"}},
		ReturnValue: "0",
		ErrorHint:   "ENOENT (DEL/MOD on fd not in epoll), EEXIST (ADD on already registered fd)",
	},
	"poll": {
		Description: "Wait for events on a set of file descriptors.",
		Signature:   "poll(fds, nfds, timeout) → n_ready",
		Args:        [][2]string{{"fds", "array of pollfd structs"}, {"nfds", "number of fds"}, {"timeout", "milliseconds (-1 = block)"}},
		ReturnValue: "number of fds with events (0 = timeout, -1 = error)",
	},
	"futex": {
		Description: "Fast user-space locking primitive — the kernel backing for mutexes and condition variables.",
		Signature:   "futex(uaddr, op, val, timeout, uaddr2, val3) → 0 or value",
		Args:        [][2]string{{"uaddr", "address of the futex word (shared between threads)"}, {"op", "FUTEX_WAIT, FUTEX_WAKE, FUTEX_LOCK_PI, …"}, {"val", "expected value (for WAIT) or wake count (for WAKE)"}},
		Notes:       "Most of the time futex stays in user space (no syscall). A syscall happens only when a thread must actually sleep or be woken. High counts suggest heavy lock contention.",
	},
	"clone": {
		Description: "Create a new process or thread, with fine-grained control over shared resources.",
		Signature:   "clone(flags, stack, ptid, ctid, regs) → child_pid",
		Args:        [][2]string{{"flags", "CLONE_THREAD, CLONE_VM, CLONE_FS, SIGCHLD, … (dozens of flags)"}, {"stack", "new stack pointer for the child (0 = copy)"}},
		ReturnValue: "child PID in parent, 0 in child",
		Notes:       "pthread_create(3) uses clone with CLONE_THREAD|CLONE_VM. fork(2) uses clone with SIGCHLD only.",
	},
	"execve": {
		Description: "Replace the current process image with a new program.",
		Signature:   "execve(pathname, argv, envp) → (does not return on success)",
		Args:        [][2]string{{"pathname", "path to executable"}, {"argv", "argument vector (NULL-terminated array)"}, {"envp", "environment strings"}},
		ReturnValue: "does not return; -1 on error",
		ErrorHint:   "ENOENT (not found), EACCES (not executable), ENOEXEC (bad ELF), ENOMEM",
	},
	"exit_group": {
		Description: "Terminate the calling thread (exit) or all threads in the thread group (exit_group).",
		Signature:   "exit_group(status) → (does not return)",
		Args:        [][2]string{{"status", "exit code (low 8 bits visible to waitpid)"}},
		Notes:       "exit_group is what libc exit(3) calls. glibc calls exit_group so all threads terminate cleanly.",
	},
	"wait4": {
		Description: "Wait for a child process to change state.",
		Signature:   "wait4(pid, status, options, rusage) → child_pid",
		Args:        [][2]string{{"pid", "-1 = any child, >0 = specific PID"}, {"options", "WNOHANG (non-blocking), WUNTRACED, WCONTINUED"}},
		ReturnValue: "PID of child that changed state (0 with WNOHANG if none ready)",
	},
	"ioctl": {
		Description: "Device-specific control operations on a file descriptor.",
		Signature:   "ioctl(fd, request, argp) → 0 or value",
		Args:        [][2]string{{"fd", "open file descriptor (device, socket, terminal, …)"}, {"request", "device-specific command code (TIOCGWINSZ, FIONREAD, …)"}, {"argp", "pointer to in/out argument"}},
		ReturnValue: "0 or a request-specific value",
		ErrorHint:   "ENOTTY (fd is not a terminal — very common when stdout is piped), EINVAL, ENODEV",
		Notes:       "ENOTTY is expected when the process checks for a TTY but is running under sudo, in a container, or with piped output. Not a real failure.",
	},
	"prctl": {
		Description: "Control various process attributes (name, seccomp, capabilities, …).",
		Signature:   "prctl(option, arg2, arg3, arg4, arg5) → 0 or value",
		Args:        [][2]string{{"option", "PR_SET_NAME, PR_SET_SECCOMP, PR_CAP_AMBIENT, PR_SET_DUMPABLE, …"}},
		ReturnValue: "0 or option-specific value",
		ErrorHint:   "EPERM (capability required), EINVAL (unknown option)",
		Notes:       "EPERM on prctl is common in containers with restricted capabilities or seccomp profiles.",
	},
	"rt_sigaction": {
		Description: "Install or query a signal handler.",
		Signature:   "rt_sigaction(signum, act, oldact, sigsetsize) → 0",
		Args:        [][2]string{{"signum", "signal number (SIGINT, SIGSEGV, …)"}, {"act", "new sigaction struct (NULL = query only)"}, {"oldact", "previous handler (NULL = discard)"}},
		ReturnValue: "0",
	},
	"rt_sigprocmask": {
		Description: "Block, unblock, or query the set of blocked signals.",
		Signature:   "rt_sigprocmask(how, set, oldset, sigsetsize) → 0",
		Args:        [][2]string{{"how", "SIG_BLOCK, SIG_UNBLOCK, SIG_SETMASK"}, {"set", "new signal mask (NULL = query)"}, {"oldset", "previous mask"}},
		Notes:       "Called very frequently by Go and pthreads runtimes around goroutine/thread switches.",
	},
	"getpid": {
		Description: "Return the process ID of the calling process.",
		Signature:   "getpid() → pid",
		Notes:       "Modern Linux caches the PID in the vDSO — this syscall may never actually enter the kernel.",
	},
	"getuid": {
		Description: "Return the real/effective user or group ID of the calling process.",
		Signature:   "getuid() → uid",
		Notes:       "Very cheap; usually cached by libc. Frequent calls suggest credential-checking code paths.",
	},
	"lseek": {
		Description: "Reposition the read/write offset of a file descriptor.",
		Signature:   "lseek(fd, offset, whence) → new_offset",
		Args:        [][2]string{{"whence", "SEEK_SET (absolute), SEEK_CUR (relative), SEEK_END (from end)"}},
		ReturnValue: "resulting file offset",
		ErrorHint:   "ESPIPE (fd is a pipe or socket — not seekable), EINVAL",
	},
	"pipe": {
		Description: "Create a unidirectional data channel (pipe) between two file descriptors.",
		Signature:   "pipe2(pipefd[2], flags) → 0",
		Args:        [][2]string{{"pipefd", "[0]=read end, [1]=write end"}, {"flags", "O_CLOEXEC, O_NONBLOCK, O_DIRECT"}},
		ReturnValue: "0",
	},
	"dup": {
		Description: "Duplicate a file descriptor.",
		Signature:   "dup2(oldfd, newfd) → newfd",
		Args:        [][2]string{{"oldfd", "fd to duplicate"}, {"newfd", "desired fd number (closed first if open)"}},
		ReturnValue: "new file descriptor",
	},
	"socket": {
		Description: "Create a communication endpoint (socket).",
		Signature:   "socket(domain, type, protocol) → fd",
		Args:        [][2]string{{"domain", "AF_INET, AF_INET6, AF_UNIX, AF_NETLINK, …"}, {"type", "SOCK_STREAM, SOCK_DGRAM, SOCK_RAW | SOCK_NONBLOCK | SOCK_CLOEXEC"}, {"protocol", "0 (auto), IPPROTO_TCP, IPPROTO_UDP, …"}},
		ReturnValue: "new socket fd",
	},
	"bind": {
		Description: "Assign a local address to a socket.",
		Signature:   "bind(sockfd, addr, addrlen) → 0",
		Args:        [][2]string{{"addr", "local address to bind (port + IP or Unix path)"}},
		ErrorHint:   "EADDRINUSE (port already in use), EACCES (port < 1024 without CAP_NET_BIND_SERVICE)",
	},
	"listen": {
		Description: "Mark a socket as passive (ready to accept connections).",
		Signature:   "listen(sockfd, backlog) → 0",
		Args:        [][2]string{{"backlog", "max length of pending connection queue"}},
	},
	"setsockopt": {
		Description: "Set or get socket options (timeouts, buffers, TCP_NODELAY, SO_REUSEADDR, …).",
		Signature:   "setsockopt(sockfd, level, optname, optval, optlen) → 0",
		Args:        [][2]string{{"level", "SOL_SOCKET, IPPROTO_TCP, IPPROTO_IP, …"}, {"optname", "SO_REUSEADDR, SO_KEEPALIVE, TCP_NODELAY, SO_RCVBUF, …"}},
	},
	"getsockname": {
		Description: "Get the local (getsockname) or remote (getpeername) address of a socket.",
		Signature:   "getsockname(sockfd, addr, addrlen) → 0",
	},
	"getrandom": {
		Description: "Obtain cryptographically secure random bytes from the kernel.",
		Signature:   "getrandom(buf, buflen, flags) → bytes_filled",
		Args:        [][2]string{{"flags", "0 (block until entropy ready), GRND_NONBLOCK, GRND_RANDOM"}},
		Notes:       "Preferred over /dev/urandom. Called at startup by TLS libraries and language runtimes for seed material.",
	},
	"statfs": {
		Description: "Get filesystem statistics (type, free space, block size, …).",
		Signature:   "statfs(pathname, buf) → 0",
		Args:        [][2]string{{"pathname", "path on the filesystem to inspect"}, {"buf", "struct statfs to fill"}},
		ErrorHint:   "ENOENT, EACCES, ENOSYS (on special filesystems like /proc)",
		Notes:       "Errors on /proc or /sys are expected — those filesystems may not support statfs.",
	},
	"fcntl": {
		Description: "Perform miscellaneous operations on a file descriptor (flags, locks, async I/O).",
		Signature:   "fcntl(fd, cmd, arg) → value",
		Args:        [][2]string{{"cmd", "F_GETFL, F_SETFL (O_NONBLOCK), F_GETFD, F_SETFD (FD_CLOEXEC), F_DUPFD, F_SETLK, …"}},
	},
	"sendfile": {
		Description: "Transfer data between two file descriptors entirely in kernel space.",
		Signature:   "sendfile(out_fd, in_fd, offset, count) → bytes_sent",
		Notes:       "Zero-copy: data never crosses user space. Used by web servers to send file contents over sockets.",
	},
	"prlimit64": {
		Description: "Get or set resource limits (CPU, memory, open files, …) for a process.",
		Signature:   "prlimit64(pid, resource, new_limit, old_limit) → 0",
		Args:        [][2]string{{"resource", "RLIMIT_NOFILE, RLIMIT_AS, RLIMIT_STACK, RLIMIT_CORE, …"}, {"pid", "0 = calling process"}},
	},
	"eventfd": {
		Description: "Create a file descriptor for event notification between threads/processes.",
		Signature:   "eventfd2(initval, flags) → fd",
		Args:        [][2]string{{"initval", "initial counter value"}, {"flags", "EFD_NONBLOCK, EFD_CLOEXEC, EFD_SEMAPHORE"}},
		Notes:       "Used by Go runtime and libuv/libevent to wake up blocked pollers without a pipe.",
	},
	"set_tid_address": {
		Description: "Set the address that the kernel will clear when the thread exits.",
		Signature:   "set_tid_address(tidptr) → tid",
		Notes:       "Called once at thread startup by glibc. Used for robust futex cleanup on thread exit.",
	},
	"arch_prctl": {
		Description: "Set architecture-specific thread state (e.g. FS/GS segment base for TLS).",
		Signature:   "arch_prctl(code, addr) → 0",
		Args:        [][2]string{{"code", "ARCH_SET_FS (set FS base for thread-local storage), ARCH_GET_FS, …"}},
		Notes:       "Called once per thread by glibc to initialise thread-local storage (TLS). Normal during startup.",
	},
}

// SyscallInfo returns human-readable reference data for well-known syscalls.
// Unknown syscalls get a generic entry.
func SyscallInfo(name string) SyscallDetail {
	if canonical, ok := SyscallAliases[name]; ok {
		name = canonical
	}

	if d, ok := SyscallDetails[name]; ok {
		return d
	}

	return SyscallDetail{
		Description: fmt.Sprintf("Kernel syscall %q — no reference entry available.", name),
		Notes:       "See 'man 2 " + name + "' for full documentation.",
	}
}
