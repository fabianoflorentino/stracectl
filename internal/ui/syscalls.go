package ui

import "fmt"

// syscallDetail holds reference information for one syscall.
type syscallDetail struct {
	description string
	signature   string
	args        [][2]string // [name, description]
	returnValue string
	errorHint   string
	notes       string
}

// syscallAliases maps variant/legacy syscall names to their canonical key in syscallDetails.
// To add a new alias, add one line here — no other changes needed.
var syscallAliases = map[string]string{
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

// syscallDetails maps canonical syscall names to human-readable reference data.
// To add coverage for a new syscall, add an entry here — no logic changes needed.
var syscallDetails = map[string]syscallDetail{
	"read": {
		description: "Read bytes from a file descriptor into a buffer.",
		signature:   "read(fd, buf, count) → bytes_read",
		args:        [][2]string{{"fd", "open file descriptor to read from"}, {"buf", "destination buffer in user space"}, {"count", "maximum number of bytes to read"}},
		returnValue: "number of bytes read (0 = EOF)",
		errorHint:   "EAGAIN (non-blocking, no data), EBADF (bad fd), EFAULT (bad buffer), EINTR (signal)",
		notes:       "High read() counts often indicate heavy file or socket data transfer. EAGAIN is expected for non-blocking I/O and not a real error.",
	},
	"write": {
		description: "Write bytes from a buffer to a file descriptor.",
		signature:   "write(fd, buf, count) → bytes_written",
		args:        [][2]string{{"fd", "open file descriptor to write to"}, {"buf", "source buffer in user space"}, {"count", "number of bytes to write"}},
		returnValue: "number of bytes written",
		errorHint:   "EAGAIN (non-blocking, buffer full), EBADF, EPIPE (peer closed), EFAULT",
		notes:       "A short write (return < count) can happen on sockets or pipes; callers should loop.",
	},
	"openat": {
		description: "Open or create a file, returning a file descriptor.",
		signature:   "openat(dirfd, pathname, flags, mode) → fd",
		args:        [][2]string{{"dirfd", "AT_FDCWD or directory fd for relative path"}, {"pathname", "path to file"}, {"flags", "O_RDONLY, O_WRONLY, O_CREAT, O_TRUNC, …"}, {"mode", "permission bits when O_CREAT is used"}},
		returnValue: "new file descriptor (≥ 0)",
		errorHint:   "ENOENT (not found), EACCES (permission), EMFILE (too many open fds), EEXIST (O_CREAT|O_EXCL)",
		notes:       "High ENOENT error rates are normal: the dynamic linker probes many paths when loading shared libraries.",
	},
	"close": {
		description: "Close a file descriptor, releasing the kernel resource.",
		signature:   "close(fd) → 0",
		args:        [][2]string{{"fd", "file descriptor to close"}},
		returnValue: "0",
		errorHint:   "EBADF (fd not open), EIO (deferred write-back failed — data may be lost)",
		notes:       "Never ignore EIO on close() — it means data was not written to disk.",
	},
	"mmap": {
		description: "Map files or devices into memory, or allocate anonymous memory.",
		signature:   "mmap(addr, length, prot, flags, fd, offset) → addr",
		args:        [][2]string{{"addr", "hint for mapping address (usually 0 = kernel decides)"}, {"length", "size in bytes"}, {"prot", "PROT_READ | PROT_WRITE | PROT_EXEC"}, {"flags", "MAP_PRIVATE, MAP_SHARED, MAP_ANONYMOUS, …"}, {"fd", "file to map, or -1 for anonymous"}, {"offset", "file offset (must be page-aligned)"}},
		returnValue: "virtual address of the mapping (hex)",
		errorHint:   "ENOMEM (out of virtual address space), EACCES, EINVAL",
		notes:       "MAP_ANONYMOUS|MAP_PRIVATE is the usual way malloc allocates large blocks from the kernel.",
	},
	"munmap": {
		description: "Remove a memory mapping created by mmap.",
		signature:   "munmap(addr, length) → 0",
		args:        [][2]string{{"addr", "start of the mapping (must be page-aligned)"}, {"length", "size to unmap"}},
		returnValue: "0",
		errorHint:   "EINVAL (addr not aligned or not mapped)",
	},
	"mprotect": {
		description: "Change memory protection attributes on a mapped region.",
		signature:   "mprotect(addr, len, prot) → 0",
		args:        [][2]string{{"addr", "page-aligned start address"}, {"len", "length in bytes"}, {"prot", "PROT_NONE | PROT_READ | PROT_WRITE | PROT_EXEC"}},
		returnValue: "0",
		errorHint:   "EACCES (e.g. trying to make a file-backed mapping writable without write permission), EINVAL",
		notes:       "Frequently called by the dynamic linker (ld.so) during library loading — setting sections read-only after relocation.",
	},
	"madvise": {
		description: "Give the kernel hints on expected memory usage patterns.",
		signature:   "madvise(addr, length, advice) → 0",
		args:        [][2]string{{"addr", "page-aligned start"}, {"length", "region size"}, {"advice", "MADV_NORMAL, MADV_SEQUENTIAL, MADV_DONTNEED, MADV_FREE, …"}},
		returnValue: "0",
		errorHint:   "EINVAL (unknown advice or bad addr/length), EACCES",
		notes:       "EACCES or EINVAL errors are informational — the kernel ignores hints it cannot honour. Not a real failure.",
	},
	"brk": {
		description: "Adjust the end of the data segment (heap boundary).",
		signature:   "brk(addr) → new_brk",
		args:        [][2]string{{"addr", "new end of heap (0 = query current value)"}},
		returnValue: "current break address",
		notes:       "Modern malloc implementations prefer mmap for large allocations; brk is used for the initial heap.",
	},
	"fstat": {
		description: "Retrieve file metadata (size, permissions, timestamps, inode).",
		signature:   "fstat(fd, statbuf) → 0  /  stat(pathname, statbuf) → 0",
		args:        [][2]string{{"fd / pathname", "file descriptor or path to inspect"}, {"statbuf", "struct stat to fill in"}},
		returnValue: "0",
		errorHint:   "ENOENT (path not found), EACCES, EBADF",
		notes:       "High fstat/stat rates are normal when an application polls file state or a web server serves many files.",
	},
	"getdents64": {
		description: "Read directory entries from an open directory file descriptor.",
		signature:   "getdents64(fd, dirp, count) → bytes_read",
		args:        [][2]string{{"fd", "directory fd"}, {"dirp", "buffer for linux_dirent64 structs"}, {"count", "buffer size"}},
		returnValue: "bytes read (0 = end of directory)",
		errorHint:   "EBADF, ENOTDIR (fd is not a directory)",
		notes:       "Used by readdir(3). High counts suggest directory scanning (e.g. file watchers, recursive search).",
	},
	"access": {
		description: "Check whether the calling process can access a file.",
		signature:   "access(pathname, mode) → 0",
		args:        [][2]string{{"pathname", "path to check"}, {"mode", "F_OK (exists?), R_OK, W_OK, X_OK"}},
		returnValue: "0 if access allowed",
		errorHint:   "ENOENT (not found — usually harmless), EACCES (permission denied)",
		notes:       "High ENOENT rates are expected: programs probe for optional config files. Not a real error.",
	},
	"connect": {
		description: "Initiate a connection on a socket.",
		signature:   "connect(sockfd, addr, addrlen) → 0",
		args:        [][2]string{{"sockfd", "open socket fd"}, {"addr", "target address (sockaddr_in, sockaddr_un, …)"}, {"addrlen", "sizeof(*addr)"}},
		returnValue: "0 on success",
		errorHint:   "ECONNREFUSED (port closed), ETIMEDOUT, ENETUNREACH, EINPROGRESS (non-blocking)",
		notes:       "Errors are common with Happy Eyeballs (RFC 8305): both IPv4 and IPv6 are tried in parallel; the loser always fails with ECONNREFUSED or ETIMEDOUT.",
	},
	"accept4": {
		description: "Accept a new incoming connection on a listening socket.",
		signature:   "accept4(sockfd, addr, addrlen, flags) → fd",
		args:        [][2]string{{"sockfd", "listening socket fd"}, {"addr", "filled with peer address"}, {"flags", "SOCK_NONBLOCK, SOCK_CLOEXEC"}},
		returnValue: "new connected socket fd",
		errorHint:   "EAGAIN (no pending connections, non-blocking), EMFILE (too many open fds)",
	},
	"recvfrom": {
		description: "Receive data from a socket.",
		signature:   "recvfrom(sockfd, buf, len, flags, src_addr, addrlen) → bytes",
		args:        [][2]string{{"sockfd", "connected or unconnected socket"}, {"buf", "receive buffer"}, {"flags", "MSG_DONTWAIT, MSG_PEEK, MSG_WAITALL, …"}},
		returnValue: "bytes received (0 = peer closed)",
		errorHint:   "EAGAIN/EWOULDBLOCK (non-blocking, no data yet — normal), ECONNRESET",
		notes:       "EAGAIN on a non-blocking socket is not a real error — the event loop will retry.",
	},
	"sendto": {
		description: "Send data through a socket.",
		signature:   "sendto(sockfd, buf, len, flags, dest_addr, addrlen) → bytes",
		args:        [][2]string{{"sockfd", "socket fd"}, {"buf", "data to send"}, {"flags", "MSG_DONTWAIT, MSG_NOSIGNAL, …"}},
		returnValue: "bytes sent",
		errorHint:   "EPIPE (peer closed — usually triggers SIGPIPE too), EAGAIN, ECONNRESET",
	},
	"epoll_wait": {
		description: "Wait for events on an epoll file descriptor.",
		signature:   "epoll_wait(epfd, events, maxevents, timeout) → n_events",
		args:        [][2]string{{"epfd", "epoll instance fd"}, {"events", "array of epoll_event to fill"}, {"maxevents", "max events to return"}, {"timeout", "ms to wait (-1 = block forever)"}},
		returnValue: "number of ready fds (0 = timeout)",
		notes:       "The main blocking call in event-driven servers (nginx, Node.js, Go net poller). High count = many I/O events.",
	},
	"epoll_ctl": {
		description: "Add, modify, or remove a file descriptor from an epoll instance.",
		signature:   "epoll_ctl(epfd, op, fd, event) → 0",
		args:        [][2]string{{"op", "EPOLL_CTL_ADD, EPOLL_CTL_MOD, EPOLL_CTL_DEL"}, {"fd", "target file descriptor"}, {"event", "epoll_event with events mask and user data"}},
		returnValue: "0",
		errorHint:   "ENOENT (DEL/MOD on fd not in epoll), EEXIST (ADD on already registered fd)",
	},
	"poll": {
		description: "Wait for events on a set of file descriptors.",
		signature:   "poll(fds, nfds, timeout) → n_ready",
		args:        [][2]string{{"fds", "array of pollfd structs"}, {"nfds", "number of fds"}, {"timeout", "milliseconds (-1 = block)"}},
		returnValue: "number of fds with events (0 = timeout, -1 = error)",
	},
	"futex": {
		description: "Fast user-space locking primitive — the kernel backing for mutexes and condition variables.",
		signature:   "futex(uaddr, op, val, timeout, uaddr2, val3) → 0 or value",
		args:        [][2]string{{"uaddr", "address of the futex word (shared between threads)"}, {"op", "FUTEX_WAIT, FUTEX_WAKE, FUTEX_LOCK_PI, …"}, {"val", "expected value (for WAIT) or wake count (for WAKE)"}},
		notes:       "Most of the time futex stays in user space (no syscall). A syscall happens only when a thread must actually sleep or be woken. High counts suggest heavy lock contention.",
	},
	"clone": {
		description: "Create a new process or thread, with fine-grained control over shared resources.",
		signature:   "clone(flags, stack, ptid, ctid, regs) → child_pid",
		args:        [][2]string{{"flags", "CLONE_THREAD, CLONE_VM, CLONE_FS, SIGCHLD, … (dozens of flags)"}, {"stack", "new stack pointer for the child (0 = copy)"}},
		returnValue: "child PID in parent, 0 in child",
		notes:       "pthread_create(3) uses clone with CLONE_THREAD|CLONE_VM. fork(2) uses clone with SIGCHLD only.",
	},
	"execve": {
		description: "Replace the current process image with a new program.",
		signature:   "execve(pathname, argv, envp) → (does not return on success)",
		args:        [][2]string{{"pathname", "path to executable"}, {"argv", "argument vector (NULL-terminated array)"}, {"envp", "environment strings"}},
		returnValue: "does not return; -1 on error",
		errorHint:   "ENOENT (not found), EACCES (not executable), ENOEXEC (bad ELF), ENOMEM",
	},
	"exit_group": {
		description: "Terminate the calling thread (exit) or all threads in the thread group (exit_group).",
		signature:   "exit_group(status) → (does not return)",
		args:        [][2]string{{"status", "exit code (low 8 bits visible to waitpid)"}},
		notes:       "exit_group is what libc exit(3) calls. glibc calls exit_group so all threads terminate cleanly.",
	},
	"wait4": {
		description: "Wait for a child process to change state.",
		signature:   "wait4(pid, status, options, rusage) → child_pid",
		args:        [][2]string{{"pid", "-1 = any child, >0 = specific PID"}, {"options", "WNOHANG (non-blocking), WUNTRACED, WCONTINUED"}},
		returnValue: "PID of child that changed state (0 with WNOHANG if none ready)",
	},
	"ioctl": {
		description: "Device-specific control operations on a file descriptor.",
		signature:   "ioctl(fd, request, argp) → 0 or value",
		args:        [][2]string{{"fd", "open file descriptor (device, socket, terminal, …)"}, {"request", "device-specific command code (TIOCGWINSZ, FIONREAD, …)"}, {"argp", "pointer to in/out argument"}},
		returnValue: "0 or a request-specific value",
		errorHint:   "ENOTTY (fd is not a terminal — very common when stdout is piped), EINVAL, ENODEV",
		notes:       "ENOTTY is expected when the process checks for a TTY but is running under sudo, in a container, or with piped output. Not a real failure.",
	},
	"prctl": {
		description: "Control various process attributes (name, seccomp, capabilities, …).",
		signature:   "prctl(option, arg2, arg3, arg4, arg5) → 0 or value",
		args:        [][2]string{{"option", "PR_SET_NAME, PR_SET_SECCOMP, PR_CAP_AMBIENT, PR_SET_DUMPABLE, …"}},
		returnValue: "0 or option-specific value",
		errorHint:   "EPERM (capability required), EINVAL (unknown option)",
		notes:       "EPERM on prctl is common in containers with restricted capabilities or seccomp profiles.",
	},
	"rt_sigaction": {
		description: "Install or query a signal handler.",
		signature:   "rt_sigaction(signum, act, oldact, sigsetsize) → 0",
		args:        [][2]string{{"signum", "signal number (SIGINT, SIGSEGV, …)"}, {"act", "new sigaction struct (NULL = query only)"}, {"oldact", "previous handler (NULL = discard)"}},
		returnValue: "0",
	},
	"rt_sigprocmask": {
		description: "Block, unblock, or query the set of blocked signals.",
		signature:   "rt_sigprocmask(how, set, oldset, sigsetsize) → 0",
		args:        [][2]string{{"how", "SIG_BLOCK, SIG_UNBLOCK, SIG_SETMASK"}, {"set", "new signal mask (NULL = query)"}, {"oldset", "previous mask"}},
		notes:       "Called very frequently by Go and pthreads runtimes around goroutine/thread switches.",
	},
	"getpid": {
		description: "Return the process ID of the calling process.",
		signature:   "getpid() → pid",
		notes:       "Modern Linux caches the PID in the vDSO — this syscall may never actually enter the kernel.",
	},
	"getuid": {
		description: "Return the real/effective user or group ID of the calling process.",
		signature:   "getuid() → uid",
		notes:       "Very cheap; usually cached by libc. Frequent calls suggest credential-checking code paths.",
	},
	"lseek": {
		description: "Reposition the read/write offset of a file descriptor.",
		signature:   "lseek(fd, offset, whence) → new_offset",
		args:        [][2]string{{"whence", "SEEK_SET (absolute), SEEK_CUR (relative), SEEK_END (from end)"}},
		returnValue: "resulting file offset",
		errorHint:   "ESPIPE (fd is a pipe or socket — not seekable), EINVAL",
	},
	"pipe": {
		description: "Create a unidirectional data channel (pipe) between two file descriptors.",
		signature:   "pipe2(pipefd[2], flags) → 0",
		args:        [][2]string{{"pipefd", "[0]=read end, [1]=write end"}, {"flags", "O_CLOEXEC, O_NONBLOCK, O_DIRECT"}},
		returnValue: "0",
	},
	"dup": {
		description: "Duplicate a file descriptor.",
		signature:   "dup2(oldfd, newfd) → newfd",
		args:        [][2]string{{"oldfd", "fd to duplicate"}, {"newfd", "desired fd number (closed first if open)"}},
		returnValue: "new file descriptor",
	},
	"socket": {
		description: "Create a communication endpoint (socket).",
		signature:   "socket(domain, type, protocol) → fd",
		args:        [][2]string{{"domain", "AF_INET, AF_INET6, AF_UNIX, AF_NETLINK, …"}, {"type", "SOCK_STREAM, SOCK_DGRAM, SOCK_RAW | SOCK_NONBLOCK | SOCK_CLOEXEC"}, {"protocol", "0 (auto), IPPROTO_TCP, IPPROTO_UDP, …"}},
		returnValue: "new socket fd",
	},
	"bind": {
		description: "Assign a local address to a socket.",
		signature:   "bind(sockfd, addr, addrlen) → 0",
		args:        [][2]string{{"addr", "local address to bind (port + IP or Unix path)"}},
		errorHint:   "EADDRINUSE (port already in use), EACCES (port < 1024 without CAP_NET_BIND_SERVICE)",
	},
	"listen": {
		description: "Mark a socket as passive (ready to accept connections).",
		signature:   "listen(sockfd, backlog) → 0",
		args:        [][2]string{{"backlog", "max length of pending connection queue"}},
	},
	"setsockopt": {
		description: "Set or get socket options (timeouts, buffers, TCP_NODELAY, SO_REUSEADDR, …).",
		signature:   "setsockopt(sockfd, level, optname, optval, optlen) → 0",
		args:        [][2]string{{"level", "SOL_SOCKET, IPPROTO_TCP, IPPROTO_IP, …"}, {"optname", "SO_REUSEADDR, SO_KEEPALIVE, TCP_NODELAY, SO_RCVBUF, …"}},
	},
	"getsockname": {
		description: "Get the local (getsockname) or remote (getpeername) address of a socket.",
		signature:   "getsockname(sockfd, addr, addrlen) → 0",
	},
	"getrandom": {
		description: "Obtain cryptographically secure random bytes from the kernel.",
		signature:   "getrandom(buf, buflen, flags) → bytes_filled",
		args:        [][2]string{{"flags", "0 (block until entropy ready), GRND_NONBLOCK, GRND_RANDOM"}},
		notes:       "Preferred over /dev/urandom. Called at startup by TLS libraries and language runtimes for seed material.",
	},
	"statfs": {
		description: "Get filesystem statistics (type, free space, block size, …).",
		signature:   "statfs(pathname, buf) → 0",
		args:        [][2]string{{"pathname", "path on the filesystem to inspect"}, {"buf", "struct statfs to fill"}},
		errorHint:   "ENOENT, EACCES, ENOSYS (on special filesystems like /proc)",
		notes:       "Errors on /proc or /sys are expected — those filesystems may not support statfs.",
	},
	"fcntl": {
		description: "Perform miscellaneous operations on a file descriptor (flags, locks, async I/O).",
		signature:   "fcntl(fd, cmd, arg) → value",
		args:        [][2]string{{"cmd", "F_GETFL, F_SETFL (O_NONBLOCK), F_GETFD, F_SETFD (FD_CLOEXEC), F_DUPFD, F_SETLK, …"}},
	},
	"sendfile": {
		description: "Transfer data between two file descriptors entirely in kernel space.",
		signature:   "sendfile(out_fd, in_fd, offset, count) → bytes_sent",
		notes:       "Zero-copy: data never crosses user space. Used by web servers to send file contents over sockets.",
	},
	"prlimit64": {
		description: "Get or set resource limits (CPU, memory, open files, …) for a process.",
		signature:   "prlimit64(pid, resource, new_limit, old_limit) → 0",
		args:        [][2]string{{"resource", "RLIMIT_NOFILE, RLIMIT_AS, RLIMIT_STACK, RLIMIT_CORE, …"}, {"pid", "0 = calling process"}},
	},
	"eventfd": {
		description: "Create a file descriptor for event notification between threads/processes.",
		signature:   "eventfd2(initval, flags) → fd",
		args:        [][2]string{{"initval", "initial counter value"}, {"flags", "EFD_NONBLOCK, EFD_CLOEXEC, EFD_SEMAPHORE"}},
		notes:       "Used by Go runtime and libuv/libevent to wake up blocked pollers without a pipe.",
	},
	"set_tid_address": {
		description: "Set the address that the kernel will clear when the thread exits.",
		signature:   "set_tid_address(tidptr) → tid",
		notes:       "Called once at thread startup by glibc. Used for robust futex cleanup on thread exit.",
	},
	"arch_prctl": {
		description: "Set architecture-specific thread state (e.g. FS/GS segment base for TLS).",
		signature:   "arch_prctl(code, addr) → 0",
		args:        [][2]string{{"code", "ARCH_SET_FS (set FS base for thread-local storage), ARCH_GET_FS, …"}},
		notes:       "Called once per thread by glibc to initialise thread-local storage (TLS). Normal during startup.",
	},
}

// syscallInfo returns human-readable reference data for well-known syscalls.
// Unknown syscalls get a generic entry.
func syscallInfo(name string) syscallDetail {
	if canonical, ok := syscallAliases[name]; ok {
		name = canonical
	}

	if d, ok := syscallDetails[name]; ok {
		return d
	}

	return syscallDetail{
		description: fmt.Sprintf("Kernel syscall %q — no reference entry available.", name),
		notes:       "See 'man 2 " + name + "' for full documentation.",
	}
}
