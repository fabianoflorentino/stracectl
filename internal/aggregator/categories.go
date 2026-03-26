package aggregator

import "encoding/json"

var (
	ioSyscalls = []string{
		"read", "write", "pread64", "pwrite64", "readv", "writev",
		"open", "openat", "close", "dup", "dup2", "dup3",
		"pipe", "pipe2", "sendfile", "copy_file_range",
	}
	fsSyscalls = []string{
		"stat", "fstat", "lstat", "newfstatat", "statfs", "fstatfs",
		"access", "faccessat", "getdents", "getdents64",
		"mkdir", "mkdirat", "rmdir", "unlink", "unlinkat",
		"rename", "renameat", "renameat2",
		"link", "linkat", "symlink", "symlinkat", "readlink", "readlinkat",
		"chmod", "fchmod", "chown", "lchown", "fchown",
		"utime", "utimes", "utimensat", "truncate", "ftruncate",
		"lseek", "llseek", "mknod", "mknodat",
		"statx", "inotify_init", "inotify_add_watch", "inotify_rm_watch",
		"fanotify_init", "fanotify_mark", "chdir", "fchdir", "getcwd",
		"mount", "umount", "umount2", "sync", "fsync", "fdatasync",
		"getxattr", "setxattr", "listxattr", "removexattr",
	}
	netSyscalls = []string{
		"socket", "bind", "listen", "accept", "accept4",
		"connect", "sendto", "recvfrom", "sendmsg", "recvmsg",
		"sendmmsg", "recvmmsg", "getsockname", "getpeername",
		"setsockopt", "getsockopt", "shutdown", "socketpair",
		"poll", "ppoll", "select", "pselect6", "epoll_create",
		"epoll_create1", "epoll_ctl", "epoll_wait", "epoll_pwait",
	}
	memSyscalls = []string{
		"mmap", "mmap2", "munmap", "mprotect", "madvise",
		"mremap", "msync", "mincore", "mlock", "munlock",
		"mlock2", "mlockall", "munlockall", "brk", "sbrk",
	}
	processSyscalls = []string{
		"clone", "clone3", "fork", "vfork", "execve", "execveat",
		"wait4", "waitpid", "waitid", "exit", "exit_group",
		"getpid", "getppid", "getpgid", "setpgid", "getsid", "setsid",
		"getuid", "geteuid", "getgid", "getegid", "getgroups",
		"setuid", "setgid", "prctl", "prlimit64", "ptrace",
		"kill", "tgkill", "tkill", "pause",
	}
	signalSyscalls = []string{
		"rt_sigaction", "rt_sigprocmask", "rt_sigreturn",
		"sigaction", "signal", "sigprocmask", "sigreturn",
		"rt_sigsuspend", "rt_sigpending", "rt_sigtimedwait",
		"signalfd", "signalfd4", "eventfd", "eventfd2",
	}
)

// syscallCategories maps each known syscall name to its Category.
var syscallCategories = func() map[string]Category {
	lists := []struct {
		cat   Category
		calls []string
	}{
		{CatIO, ioSyscalls},
		{CatFS, fsSyscalls},
		{CatNet, netSyscalls},
		{CatMem, memSyscalls},
		{CatProcess, processSyscalls},
		{CatSignal, signalSyscalls},
	}

	m := make(map[string]Category)
	for _, l := range lists {
		for _, name := range l.calls {
			m[name] = l.cat
		}
	}

	return m
}()

func classify(name string) Category {
	if c, ok := syscallCategories[name]; ok {
		return c
	}

	return CatOther
}

// tiny helpers to avoid importing encoding/json in types.go
func jsonMarshalString(s string) ([]byte, error) {
	return json.Marshal(s)
}

func jsonUnmarshalString(data []byte, v *string) error {
	return json.Unmarshal(data, v)
}
