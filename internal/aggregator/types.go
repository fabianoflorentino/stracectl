package aggregator

import (
	"sort"
	"time"
)

// Category groups syscalls by purpose.
type Category int

const (
	CatOther Category = iota
	CatIO
	CatFS
	CatNet
	CatMem
	CatProcess
	CatSignal
)

const (
	SortByCount SortField = iota
	SortByTotal
	SortByAvg
	SortByMin
	SortByMax
	SortByErrors
	SortByName
	SortByCategory
)

const (
	// caps and sizes used across aggregator
	maxErrorSamples = 10
	maxLogEntries   = 500
)

// LogEntry represents a single syscall event with its timestamp,
// PID, name, arguments, return value and error (if any).
type LogEntry struct {
	Time   time.Time
	PID    int
	Name   string
	Args   string
	RetVal string
	Error  string
}

// ErrorSample represents a recent error occurrence for a syscall,
// including the arguments, errno and timestamp.
type ErrorSample struct {
	Args  string
	Errno string
	Time  time.Time
}

// ErrnoCount represents a specific errno and the count of occurrences for that errno.
// This is used in error breakdowns to show which errors are most common for a syscall.
type ErrnoCount struct {
	Errno string
	Count int64
}

// FileStat represents a path and its observed open count.
type FileStat struct {
	Path  string `json:"path"`
	Count int64  `json:"count"`
}

// SortField selects the column used when sorting Sorted output.
type SortField int

// CategoryStats holds per-category totals for the summary bar.
type CategoryStats struct {
	Count int64
	Errs  int64
}

// SyscallStat holds aggregated statistics for a specific syscall
// name, including counts, latencies, error breakdowns and file attribution.
// When the aggregator is running in per-PID mode, PID is set to the traced
// process ID so callers can distinguish per-process rows.
type SyscallStat struct {
	Name           string
	PID            int // non-zero only in per-PID mode
	Category       Category
	Count          int64
	Errors         int64
	TotalTime      time.Duration
	MinTime        time.Duration
	MaxTime        time.Duration
	P95            time.Duration
	P99            time.Duration
	ErrRate60s     int64
	Files          []FileStat
	ErrorBreakdown map[string]int64
	RecentErrors   []ErrorSample
	latHist        [latencyBuckets]int64
	errWin         errWindow
}

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

// MarshalJSON implements json.Marshaler so Category is serialized as a string in JSON output.
// This makes the JSON output more human-readable and easier to work with in tools that consume it.
func (c Category) MarshalJSON() ([]byte, error) {
	return jsonMarshalString(c.String())
}

// UnmarshalJSON implements json.Unmarshaler to allow parsing Category from JSON strings.
// It accepts the same string values as produced by MarshalJSON.
func (c *Category) UnmarshalJSON(data []byte) error {
	var s string

	if err := jsonUnmarshalString(data, &s); err != nil {
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

// AvgTime returns the average latency for the syscall, computed as TotalTime divided by Count.
func (s *SyscallStat) AvgTime() time.Duration {
	if s.Count == 0 {
		return 0
	}

	return s.TotalTime / time.Duration(s.Count)
}

// ErrPct returns the error percentage for the syscall, computed as (Errors / Count) * 100.
func (s *SyscallStat) ErrPct() float64 {
	if s.Count == 0 {
		return 0
	}

	return float64(s.Errors) / float64(s.Count) * 100
}

// sortErrnoCount sorts a slice of ErrnoCount first by count descending, then by errno name ascending for ties.
// This is used to present error breakdowns in a consistent and human-friendly order.
func sortErrnoCount(s []ErrnoCount) {
	var sortCount = func(i, j int) bool {
		if s[i].Count != s[j].Count {
			return s[i].Count > s[j].Count
		}

		return s[i].Errno < s[j].Errno
	}

	sort.Slice(s, sortCount)
}
