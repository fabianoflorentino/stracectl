// Package aggregator accumulates SyscallEvent values and provides sorted views.
package aggregator

import (
	"encoding/json"
	"math/bits"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fabianoflorentino/stracectl/internal/models"
	"github.com/fabianoflorentino/stracectl/internal/procinfo"
)

// ProcInfo and ReadProcInfo moved to internal/procinfo for single responsibility
// and easier testing. Use procinfo.Read(pid) to obtain a procinfo.ProcInfo.

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
	// ErrRate60s is the number of errors recorded in the last 60 seconds.
	// Populated by Sorted() and Get().
	ErrRate60s int64
	// Files holds top file paths associated with this syscall (populated by Sorted()).
	Files []FileStat
	// ErrorBreakdown counts occurrences of each distinct errno (e.g. "ENOENT").
	// It is non-nil only when at least one error has been recorded.
	ErrorBreakdown map[string]int64
	// RecentErrors is a ring buffer of the last maxErrorSamples failed calls.
	RecentErrors []ErrorSample
	// latHist is an unexported log2 histogram used to compute P95/P99.
	latHist [latencyBuckets]int64
	// errWin is an unexported sliding-window error rate tracker.
	errWin errWindow
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

const (
	maxErrorSamples = 10  // max recent error samples retained per syscall
	maxLogEntries   = 500 // max raw events kept in the live log ring buffer
	// fileStatsCap limits distinct tracked file paths to avoid unbounded memory usage.
	fileStatsCap = 10_000
	// maxPathLen truncates observed paths to a safe maximum length.
	maxPathLen = 1024
)

// LogEntry is one line in the live-log ring buffer.
type LogEntry struct {
	Time   time.Time
	PID    int
	Name   string
	Args   string
	RetVal string
	Error  string
}

// errWindowSize is the number of 1-second buckets kept for the sliding error-rate window.
const errWindowSize = 60

// errWindow is a circular buffer that counts errors per second over the last 60 s.
// bucket[i] holds the error count for the Unix second (i % errWindowSize).
// epoch[i] records which Unix second that bucket belongs to; stale buckets are zeroed.
type errWindow struct {
	buckets [errWindowSize]int32
	epochs  [errWindowSize]int64 // Unix seconds
}

// record adds one error at the given Unix second.
func (w *errWindow) record(sec int64) {
	idx := int(sec % errWindowSize)

	if w.epochs[idx] != sec {
		// New second: reset the stale bucket.
		w.buckets[idx] = 0
		w.epochs[idx] = sec
	}

	w.buckets[idx]++
}

// sum returns the total errors in the last 60 s relative to now (Unix second).
func (w *errWindow) sum(now int64) int64 {
	var total int64

	for i := range errWindowSize {
		if now-w.epochs[i] < errWindowSize {
			total += int64(w.buckets[i])
		}
	}

	return total
}

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

	for i := range latencyBuckets {
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
	SortByMin                       // lowest min latency first
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
	mu        sync.RWMutex
	stats     map[string]*SyscallStat
	total     int64
	errors    int64
	started   time.Time
	prevRate  rateSnapshot
	rate      float64 // syscalls/s, updated every snapshot
	procInfo  procinfo.ProcInfo
	logBuf    []LogEntry // ring buffer of recent raw events
	fileStats map[string]int64
	// fileStatsByCall maps syscall name -> (path -> count)
	fileStatsByCall map[string]map[string]int64
	// fdToPath maps pid -> fd -> path, used to attribute fd-based syscalls to paths
	fdToPath map[int]map[int]string
	done     bool // true when the traced process has exited
}

type rateSnapshot struct {
	total int64
	at    time.Time
}

func New() *Aggregator {
	now := time.Now()

	return &Aggregator{
		stats:           make(map[string]*SyscallStat),
		fileStats:       make(map[string]int64),
		fileStatsByCall: make(map[string]map[string]int64),
		fdToPath:        make(map[int]map[int]string),
		started:         now,
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
		// sliding window: record error in the 1-second bucket
		sec := e.Time.Unix()
		if sec == 0 {
			sec = time.Now().Unix()
		}
		s.errWin.record(sec)
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

	// Append to live-log ring buffer.
	entry := LogEntry{Time: e.Time, PID: e.PID, Name: e.Name, Args: e.Args, RetVal: e.RetVal, Error: e.Error}
	if len(a.logBuf) < maxLogEntries {
		a.logBuf = append(a.logBuf, entry)
	} else {
		copy(a.logBuf, a.logBuf[1:])
		a.logBuf[maxLogEntries-1] = entry
	}

	// Attribute observed events to file paths when possible.
	// 1) If the syscall arguments contain a quoted path, use it directly.
	// 2) Otherwise, try to attribute by file descriptor using fd->path mappings
	//    maintained from successful open/openat calls.
	if p := extractPathFromArgs(e.Name, e.Args); p != "" {
		if len(p) > maxPathLen {
			p = p[:maxPathLen]
		}
		if a.fileStats == nil {
			a.fileStats = make(map[string]int64)
		}
		if len(a.fileStats) < fileStatsCap || a.fileStats[p] > 0 {
			a.fileStats[p]++
		}
		if a.fileStatsByCall == nil {
			a.fileStatsByCall = make(map[string]map[string]int64)
		}
		if a.fileStatsByCall[e.Name] == nil {
			a.fileStatsByCall[e.Name] = make(map[string]int64)
		}
		if len(a.fileStatsByCall[e.Name]) < fileStatsCap || a.fileStatsByCall[e.Name][p] > 0 {
			a.fileStatsByCall[e.Name][p]++
		}

		// If this was an open/openat and it returned a valid fd, map fd->path
		if (e.Name == "open" || e.Name == "openat") && e.RetVal != "" {
			if fd, ok := parseRetInt(e.RetVal); ok && fd >= 0 {
				if a.fdToPath == nil {
					a.fdToPath = make(map[int]map[int]string)
				}
				if a.fdToPath[e.PID] == nil {
					a.fdToPath[e.PID] = make(map[int]string)
				}
				a.fdToPath[e.PID][fd] = p
			}
		}
	} else {
		// No explicit path in args: try to attribute by file descriptor where possible.
		if fd, ok := parseFirstIntArg(e.Args); ok {
			if a.fdToPath != nil {
				if m := a.fdToPath[e.PID]; m != nil {
					if path, ok2 := m[fd]; ok2 && path != "" {
						if a.fileStatsByCall == nil {
							a.fileStatsByCall = make(map[string]map[string]int64)
						}
						if a.fileStatsByCall[e.Name] == nil {
							a.fileStatsByCall[e.Name] = make(map[string]int64)
						}
						if len(a.fileStatsByCall[e.Name]) < fileStatsCap || a.fileStatsByCall[e.Name][path] > 0 {
							a.fileStatsByCall[e.Name][path]++
						}
					}
				}
			}
		}

		// Special-case descriptor-moving syscalls: dup/dup2/dup3 copy fd mappings.
		switch e.Name {
		case "dup", "dup2", "dup3":
			if oldfd, ok := parseFirstIntArg(e.Args); ok {
				if newfd, ok2 := parseRetInt(e.RetVal); ok2 && newfd >= 0 {
					if a.fdToPath == nil {
						a.fdToPath = make(map[int]map[int]string)
					}
					if a.fdToPath[e.PID] == nil {
						a.fdToPath[e.PID] = make(map[int]string)
					}
					if path, ok3 := a.fdToPath[e.PID][oldfd]; ok3 && path != "" {
						a.fdToPath[e.PID][newfd] = path
					}
				}
			}
		case "close":
			if fd, ok := parseFirstIntArg(e.Args); ok {
				if a.fdToPath != nil {
					if m := a.fdToPath[e.PID]; m != nil {
						if path, ok2 := m[fd]; ok2 && path != "" {
							if a.fileStatsByCall == nil {
								a.fileStatsByCall = make(map[string]map[string]int64)
							}
							if a.fileStatsByCall[e.Name] == nil {
								a.fileStatsByCall[e.Name] = make(map[string]int64)
							}
							if len(a.fileStatsByCall[e.Name]) < fileStatsCap || a.fileStatsByCall[e.Name][path] > 0 {
								a.fileStatsByCall[e.Name][path]++
							}
						}
						// Remove mapping on close to avoid stale entries.
						delete(m, fd)
					}
				}
			}
		}
	}
}

// FileStat represents a path and its observed open count.
type FileStat struct {
	Path  string `json:"path"`
	Count int64  `json:"count"`
}

// TopFiles returns the top N files by observed open count. Pass n<=0 to return all.
func (a *Aggregator) TopFiles(n int) []FileStat {
	a.mu.RLock()
	defer a.mu.RUnlock()

	out := make([]FileStat, 0, len(a.fileStats))
	for p, c := range a.fileStats {
		out = append(out, FileStat{Path: p, Count: c})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Count > out[j].Count })
	if n > 0 && len(out) > n {
		out = out[:n]
	}
	return out
}

// TopFilesForSyscall returns the top N files observed for a specific syscall name.
// Pass n<=0 to return all.
func (a *Aggregator) TopFilesForSyscall(name string, n int) []FileStat {
	a.mu.RLock()
	defer a.mu.RUnlock()

	m := a.fileStatsByCall[name]
	if m == nil {
		return nil
	}

	out := make([]FileStat, 0, len(m))
	for p, c := range m {
		out = append(out, FileStat{Path: p, Count: c})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Count > out[j].Count })
	if n > 0 && len(out) > n {
		out = out[:n]
	}
	return out
}

// extractPathFromArgs attempts to heuristically extract the first path-like
// argument from a strace-style syscall args string. It prefers quoted strings
// and falls back to comma-splitting for open/openat.
func extractPathFromArgs(name, args string) string {
	// Only attempt quoted-path extraction for syscalls that take a pathname
	// as an argument. This avoids misinterpreting arbitrary byte buffers
	// (e.g. read() payloads) as file paths.
	pathSyscalls := map[string]bool{
		"open": true, "openat": true, "creat": true,
		"stat": true, "fstat": true, "lstat": true, "newfstatat": true, "statx": true,
		"access": true, "faccessat": true,
		"execve": true, "execveat": true,
		"readlink": true, "readlinkat": true,
		"symlink": true, "symlinkat": true,
		"unlink": true, "unlinkat": true,
		"rename": true, "renameat": true, "renameat2": true,
		"link": true, "linkat": true,
		"mkdir": true, "mkdirat": true, "rmdir": true,
		"chdir": true,
	}

	if !pathSyscalls[name] {
		return ""
	}

	// 1) look for a quoted string "..." and unescape it
	if i := strings.Index(args, "\""); i >= 0 {
		if j := strings.Index(args[i+1:], "\""); j >= 0 {
			s := args[i+1 : i+1+j]
			return unescapePath(s)
		}
	}

	// 2) fallback: split by commas and pick the likely argument for open/openat-like calls
	parts := strings.SplitN(args, ",", 3)
	var cand string
	switch name {
	case "open":
		if len(parts) >= 1 {
			cand = strings.TrimSpace(parts[0])
		}
	case "openat":
		if len(parts) >= 2 {
			cand = strings.TrimSpace(parts[1])
		}
	case "creat":
		if len(parts) >= 1 {
			cand = strings.TrimSpace(parts[0])
		}
	default:
		// For other path-taking syscalls, we don't attempt the numeric fallback.
	}
	// sanitize common non-path tokens (NULL or numeric/pointer-like values)
	if cand == "" || cand == "NULL" || cand == "0" || strings.HasPrefix(cand, "0x") {
		return ""
	}
	// If the candidate is quoted (e.g. '"/path"'), strip quotes and unescape.
	if strings.HasPrefix(cand, "\"") && strings.HasSuffix(cand, "\"") {
		return unescapePath(cand[1 : len(cand)-1])
	}
	return cand
}

// unescapePath attempts to handle C-style escapes using strconv.Unquote.
func unescapePath(s string) string {
	if unq, err := strconv.Unquote("\"" + s + "\""); err == nil {
		// Reject strings that contain control characters (including NUL)
		// as they are very likely to be binary payloads rather than paths.
		for _, r := range unq {
			if r == '\x00' || (r < 32 && r != '\t') {
				return ""
			}
		}
		return unq
	}
	// If Unquote failed, fall back to returning the raw input only if it
	// doesn't contain control characters.
	for _, r := range s {
		if r == '\x00' || (r < 32 && r != '\t') {
			return ""
		}
	}
	return s
}

// parseFirstIntArg attempts to parse the first comma-separated argument as an int.
// Returns (value, true) on success.
func parseFirstIntArg(args string) (int, bool) {
	parts := strings.SplitN(args, ",", 2)
	if len(parts) == 0 {
		return 0, false
	}
	s := strings.TrimSpace(parts[0])
	if s == "" {
		return 0, false
	}
	if v, err := strconv.Atoi(s); err == nil {
		return v, true
	}
	return 0, false
}

// parseRetInt parses a syscall return value (decimal or hex) into int.
func parseRetInt(ret string) (int, bool) {
	if ret == "" {
		return 0, false
	}
	if v, err := strconv.Atoi(ret); err == nil {
		return v, true
	}
	if strings.HasPrefix(ret, "0x") {
		if v, err := strconv.ParseInt(ret, 0, 0); err == nil {
			return int(v), true
		}
	}
	return 0, false
}

// Sorted returns a copy of all stats sorted by the given field.
func (a *Aggregator) Sorted(by SortField) []SyscallStat {
	// Snapshot current stats with minimal lock hold. We copy the syscall
	// structs and any referenced maps/slices we need to read without holding
	// the lock, then compute percentiles and the per-syscall top file
	// outside the critical section. This prevents long-running reader work
	// (sorting, scanning) from blocking writers calling Add().
	a.mu.RLock()
	statsCopy := make([]SyscallStat, 0, len(a.stats))
	// snapshot of fileStatsByCall: name -> (path -> count)
	fileMapSnap := make(map[string]map[string]int64, len(a.fileStatsByCall))
	for name, s := range a.stats {
		// shallow copy of the struct (copies arrays; slices and maps remain shared)
		cp := *s

		// Deep-copy ErrorBreakdown map to avoid races when released.
		if s.ErrorBreakdown != nil {
			cp.ErrorBreakdown = make(map[string]int64, len(s.ErrorBreakdown))
			for k, v := range s.ErrorBreakdown {
				cp.ErrorBreakdown[k] = v
			}
		}

		// Deep-copy RecentErrors slice
		if len(s.RecentErrors) > 0 {
			cp.RecentErrors = make([]ErrorSample, len(s.RecentErrors))
			copy(cp.RecentErrors, s.RecentErrors)
		}

		statsCopy = append(statsCopy, cp)

		if m := a.fileStatsByCall[name]; m != nil {
			mm := make(map[string]int64, len(m))
			for k, v := range m {
				mm[k] = v
			}
			fileMapSnap[name] = mm
		}
	}
	a.mu.RUnlock()

	out := make([]SyscallStat, 0, len(statsCopy))
	now := time.Now().Unix()
	for _, cp := range statsCopy {
		// compute percentiles from the copied histogram
		cp.P95 = latPercentile(&cp.latHist, 95)
		cp.P99 = latPercentile(&cp.latHist, 99)
		cp.ErrRate60s = cp.errWin.sum(now)

		// Attach only the top-1 file (if any) observed for this syscall by
		// scanning the snapshot map; avoid full sorting which is expensive.
		if mm, ok := fileMapSnap[cp.Name]; ok && len(mm) > 0 {
			var bestPath string
			var bestCount int64
			for p, c := range mm {
				if c > bestCount {
					bestCount = c
					bestPath = p
				}
			}
			if bestPath != "" {
				cp.Files = []FileStat{{Path: bestPath, Count: bestCount}}
			} else {
				cp.Files = nil
			}
		} else {
			cp.Files = nil
		}

		out = append(out, cp)
	}

	sort.Slice(out, func(i, j int) bool {
		switch by {
		case SortByTotal:
			return out[i].TotalTime > out[j].TotalTime
		case SortByAvg:
			return out[i].AvgTime() > out[j].AvgTime()
		case SortByMin:
			// Sort by lowest min latency (ascending, so smallest first)
			return out[i].MinTime < out[j].MinTime
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
	cp.ErrRate60s = s.errWin.sum(time.Now().Unix())

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

// SetProcInfo stores process metadata for the traced process.
func (a *Aggregator) SetProcInfo(info procinfo.ProcInfo) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.procInfo = info
}

// GetProcInfo returns the stored process metadata.
func (a *Aggregator) GetProcInfo() procinfo.ProcInfo {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.procInfo
}

// RecentLog returns a copy of the live-log ring buffer (oldest first).
func (a *Aggregator) RecentLog() []LogEntry {
	a.mu.RLock()
	defer a.mu.RUnlock()

	out := make([]LogEntry, len(a.logBuf))
	copy(out, a.logBuf)

	return out
}

// SetDone marks the traced process as having exited.
func (a *Aggregator) SetDone() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.done = true
}

// IsDone reports whether the traced process has exited.
func (a *Aggregator) IsDone() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.done
}
