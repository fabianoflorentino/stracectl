// Package aggregator accumulates SyscallEvent values and provides sorted views.
package aggregator

import (
	"sort"
	"sync"
	"time"

	"github.com/fabianoflorentino/stracectl/internal/models"
)

// Category groups syscalls by purpose.
type Category int

const (
	CatOther   Category = iota
	CatIO               // file descriptor read/write/open/close
	CatFS               // filesystem metadata
	CatNet              // socket/connect/send/recv
	CatMem              // mmap/mprotect/brk
	CatProcess          // clone/fork/execve/wait
	CatSignal           // signal management
)

func (c Category) String() string {
	switch c {
	case CatIO:
		return "I/O"
	case CatFS:
		return "FS"
	case CatNet:
		return "NET"
	case CatMem:
		return "MEM"
	case CatProcess:
		return "PROC"
	case CatSignal:
		return "SIG"
	default:
		return "OTHER"
	}
}

// classify maps a syscall name to its category.
func classify(name string) Category {
	switch name {
	case "read", "write", "pread64", "pwrite64", "readv", "writev",
		"open", "openat", "close", "dup", "dup2", "dup3",
		"pipe", "pipe2", "sendfile", "copy_file_range":
		return CatIO
	case "stat", "fstat", "lstat", "newfstatat", "statfs", "fstatfs",
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
		"getxattr", "setxattr", "listxattr", "removexattr":
		return CatFS
	case "socket", "bind", "listen", "accept", "accept4",
		"connect", "sendto", "recvfrom", "sendmsg", "recvmsg",
		"sendmmsg", "recvmmsg", "getsockname", "getpeername",
		"setsockopt", "getsockopt", "shutdown", "socketpair",
		"poll", "ppoll", "select", "pselect6", "epoll_create",
		"epoll_create1", "epoll_ctl", "epoll_wait", "epoll_pwait":
		return CatNet
	case "mmap", "mmap2", "munmap", "mprotect", "madvise",
		"mremap", "msync", "mincore", "mlock", "munlock",
		"mlock2", "mlockall", "munlockall", "brk", "sbrk":
		return CatMem
	case "clone", "clone3", "fork", "vfork", "execve", "execveat",
		"wait4", "waitpid", "waitid", "exit", "exit_group",
		"getpid", "getppid", "getpgid", "setpgid", "getsid", "setsid",
		"getuid", "geteuid", "getgid", "getegid", "getgroups",
		"setuid", "setgid", "prctl", "prlimit64", "ptrace",
		"kill", "tgkill", "tkill", "pause":
		return CatProcess
	case "rt_sigaction", "rt_sigprocmask", "rt_sigreturn",
		"sigaction", "signal", "sigprocmask", "sigreturn",
		"rt_sigsuspend", "rt_sigpending", "rt_sigtimedwait",
		"signalfd", "signalfd4", "eventfd", "eventfd2":
		return CatSignal
	default:
		return CatOther
	}
}

// SyscallStat holds aggregated statistics for a single syscall name.
type SyscallStat struct {
	Name     string
	Category Category
	Count    int64
	Errors   int64
	// Time spent inside the kernel (from strace -T).
	TotalTime time.Duration
	MinTime   time.Duration
	MaxTime   time.Duration
}

// AvgTime returns the mean latency per call.
func (s *SyscallStat) AvgTime() time.Duration {
	if s.Count == 0 {
		return 0
	}
	return s.TotalTime / time.Duration(s.Count)
}

// ErrPct returns error percentage 0-100.
func (s *SyscallStat) ErrPct() float64 {
	if s.Count == 0 {
		return 0
	}
	return float64(s.Errors) / float64(s.Count) * 100
}

// SortField selects the column used when sorting Sorted output.
type SortField int

const (
	SortByCount  SortField = iota // default: most frequent first
	SortByTotal                   // highest cumulative time first
	SortByAvg                     // highest average latency first
	SortByMax                     // highest peak latency first
	SortByErrors                  // most errors first
	SortByName                    // alphabetical
)

// CategoryStats holds per-category totals for the summary bar.
type CategoryStats struct {
	Count int64
	Errs  int64
}

// Aggregator is safe for concurrent use.
type Aggregator struct {
	mu       sync.RWMutex
	stats    map[string]*SyscallStat
	total    int64
	errors   int64
	started  time.Time
	prevRate rateSnapshot
	rate     float64 // syscalls/s, updated every snapshot
}

type rateSnapshot struct {
	total int64
	at    time.Time
}

func New() *Aggregator {
	now := time.Now()
	return &Aggregator{
		stats:   make(map[string]*SyscallStat),
		started: now,
		prevRate: rateSnapshot{
			total: 0,
			at:    now,
		},
	}
}

// Add records one event.
func (a *Aggregator) Add(e models.SyscallEvent) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.total++

	s, ok := a.stats[e.Name]
	if !ok {
		s = &SyscallStat{Name: e.Name, Category: classify(e.Name)}
		a.stats[e.Name] = s
	}

	s.Count++
	s.TotalTime += e.Latency

	if e.Latency > 0 {
		if s.MinTime == 0 || e.Latency < s.MinTime {
			s.MinTime = e.Latency
		}
		if e.Latency > s.MaxTime {
			s.MaxTime = e.Latency
		}
	}

	if e.IsError() {
		s.Errors++
		a.errors++
	}

	// Update rate roughly every 500ms without spawning goroutines.
	now := time.Now()
	if now.Sub(a.prevRate.at) >= 500*time.Millisecond {
		dt := now.Sub(a.prevRate.at).Seconds()
		if dt > 0 {
			a.rate = float64(a.total-a.prevRate.total) / dt
		}
		a.prevRate = rateSnapshot{total: a.total, at: now}
	}
}

// Sorted returns a copy of all stats sorted by the given field.
func (a *Aggregator) Sorted(by SortField) []SyscallStat {
	a.mu.RLock()
	defer a.mu.RUnlock()

	out := make([]SyscallStat, 0, len(a.stats))
	for _, s := range a.stats {
		out = append(out, *s)
	}

	sort.Slice(out, func(i, j int) bool {
		switch by {
		case SortByTotal:
			return out[i].TotalTime > out[j].TotalTime
		case SortByAvg:
			return out[i].AvgTime() > out[j].AvgTime()
		case SortByMax:
			return out[i].MaxTime > out[j].MaxTime
		case SortByErrors:
			return out[i].Errors > out[j].Errors
		case SortByName:
			return out[i].Name < out[j].Name
		default: // SortByCount
			return out[i].Count > out[j].Count
		}
	})

	return out
}

// CategoryBreakdown returns counts grouped by category.
func (a *Aggregator) CategoryBreakdown() map[Category]CategoryStats {
	a.mu.RLock()
	defer a.mu.RUnlock()

	m := make(map[Category]CategoryStats)
	for _, s := range a.stats {
		cs := m[s.Category]
		cs.Count += s.Count
		cs.Errs += s.Errors
		m[s.Category] = cs
	}
	return m
}

func (a *Aggregator) Total() int64 {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.total
}

func (a *Aggregator) Errors() int64 {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.errors
}

func (a *Aggregator) UniqueCount() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.stats)
}

// Rate returns the recent syscalls-per-second rate.
func (a *Aggregator) Rate() float64 {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.rate
}
