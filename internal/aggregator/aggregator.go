// Package aggregator accumulates SyscallEvent values and provides sorted views.
package aggregator

import (
	"encoding/json"
	"math/bits"
	"sort"
	"sync"
	"time"

	"github.com/fabianoflorentino/stracectl/internal/models"
)

// sortErrnoCount sorts a slice of ErrnoCount descending by count, then ascending by name.
func sortErrnoCount(s []ErrnoCount) {
	sort.Slice(s, func(i, j int) bool {
		if s[i].Count != s[j].Count {
			return s[i].Count > s[j].Count
		}
		return s[i].Errno < s[j].Errno
	})
}

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

// MarshalJSON implements json.Marshaler so Category serializes as its string
// label (e.g. "I/O") rather than as a raw integer.
func (c Category) MarshalJSON() ([]byte, error) {
	return json.Marshal(c.String())
}

// UnmarshalJSON implements json.Unmarshaler so Category round-trips through JSON
// as its string label.
func (c *Category) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	switch s {
	case "I/O":
		*c = CatIO
	case "FS":
		*c = CatFS
	case "NET":
		*c = CatNet
	case "MEM":
		*c = CatMem
	case "PROC":
		*c = CatProcess
	case "SIG":
		*c = CatSignal
	default:
		*c = CatOther
	}
	return nil
}

// syscall lists per category — data only, no logic here.
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
	// P95 and P99 are approximate latency percentiles derived from the log2 histogram.
	// They are populated by Sorted() and Get() — not during Add().
	P95 time.Duration
	P99 time.Duration
	// ErrorBreakdown counts occurrences of each distinct errno (e.g. "ENOENT").
	// It is non-nil only when at least one error has been recorded.
	ErrorBreakdown map[string]int64
	// RecentErrors is a ring buffer of the last maxErrorSamples failed calls.
	RecentErrors []ErrorSample
	// latHist is an unexported log2 histogram used to compute P95/P99.
	latHist [latencyBuckets]int64
}

// TopErrors returns the errno breakdown sorted descending by count.
// It returns at most n entries; pass 0 for all.
func (s *SyscallStat) TopErrors(n int) []ErrnoCount {
	if len(s.ErrorBreakdown) == 0 {
		return nil
	}
	out := make([]ErrnoCount, 0, len(s.ErrorBreakdown))
	for errno, cnt := range s.ErrorBreakdown {
		out = append(out, ErrnoCount{Errno: errno, Count: cnt})
	}
	sortErrnoCount(out)
	if n > 0 && len(out) > n {
		out = out[:n]
	}
	return out
}

const maxErrorSamples = 10 // max recent error samples retained per syscall

// latencyBuckets is the number of log2 histogram buckets used for percentile estimation.
// Bucket i covers the latency range [2^i, 2^(i+1)) nanoseconds.
// Using 63 buckets keeps all indices within int64 range (max bucket = 2^62 ≈ 146 years).
const latencyBuckets = 63

// latBucket returns the histogram bucket index for a nanosecond latency value.
func latBucket(ns int64) int {
	if ns <= 1 {
		return 0
	}
	b := bits.Len64(uint64(ns)) - 1
	if b >= latencyBuckets {
		b = latencyBuckets - 1
	}
	return b
}

// latPercentile returns the p-th percentile (0–100) from a log2 latency histogram.
// The result is the lower bound of the bucket that contains the p-th observation.
// Returns 0 when no positive-latency observations have been recorded.
func latPercentile(hist *[latencyBuckets]int64, p float64) time.Duration {
	var total int64
	for _, c := range hist {
		total += c
	}
	if total == 0 {
		return 0
	}
	target := total * int64(p) / 100
	if target == 0 {
		target = 1
	}
	var acc int64
	for i := 0; i < latencyBuckets; i++ {
		acc += hist[i]
		if acc >= target {
			return time.Duration(int64(1) << uint(i))
		}
	}
	return time.Duration(int64(1) << 62) // unreachable in practice
}

// ErrorSample captures args and context of a single failed syscall call.
type ErrorSample struct {
	Args  string
	Errno string
	Time  time.Time
}

// ErrnoCount pairs an errno name with its occurrence count.
type ErrnoCount struct {
	Errno string
	Count int64
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
	SortByCount    SortField = iota // default: most frequent first
	SortByTotal                     // highest cumulative time first
	SortByAvg                       // highest average latency first
	SortByMax                       // highest peak latency first
	SortByErrors                    // most errors first
	SortByName                      // alphabetical
	SortByCategory                  // grouped by category, then by count
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
		s.latHist[latBucket(int64(e.Latency))]++
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
		if e.Error != "" {
			if s.ErrorBreakdown == nil {
				s.ErrorBreakdown = make(map[string]int64)
			}
			s.ErrorBreakdown[e.Error]++
		}
		// ring buffer: keep the most recent maxErrorSamples samples
		sample := ErrorSample{Args: e.Args, Errno: e.Error, Time: e.Time}
		if len(s.RecentErrors) < maxErrorSamples {
			s.RecentErrors = append(s.RecentErrors, sample)
		} else {
			// shift left and append
			copy(s.RecentErrors, s.RecentErrors[1:])
			s.RecentErrors[maxErrorSamples-1] = sample
		}
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
		cp := *s
		cp.P95 = latPercentile(&s.latHist, 95)
		cp.P99 = latPercentile(&s.latHist, 99)
		out = append(out, cp)
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
		case SortByCategory:
			if out[i].Category != out[j].Category {
				return out[i].Category < out[j].Category
			}
			return out[i].Count > out[j].Count
		default: // SortByCount
			return out[i].Count > out[j].Count
		}
	})

	return out
}

// Get returns the aggregated stat for a single syscall by name.
// Returns false if no events have been recorded for that name.
func (a *Aggregator) Get(name string) (SyscallStat, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	s, ok := a.stats[name]
	if !ok {
		return SyscallStat{}, false
	}
	cp := *s
	cp.P95 = latPercentile(&s.latHist, 95)
	cp.P99 = latPercentile(&s.latHist, 99)
	return cp, true
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

// StartTime returns the time the aggregator was created.
func (a *Aggregator) StartTime() time.Time {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.started
}
