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

// MarshalJSON implements json.Marshaler so Category serializes as its string label.
func (c Category) MarshalJSON() ([]byte, error) {
	return jsonMarshalString(c.String())
}

// UnmarshalJSON implements json.Unmarshaler so Category round-trips through JSON.
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

// sortErrnoCount sorts a slice of ErrnoCount descending by count, then ascending by name.
func sortErrnoCount(s []ErrnoCount) {
	var sortCount = func(i, j int) bool {
		if s[i].Count != s[j].Count {
			return s[i].Count > s[j].Count
		}

		return s[i].Errno < s[j].Errno
	}

	sort.Slice(s, sortCount)
}

// SyscallStat holds aggregated statistics for a single syscall name.
type SyscallStat struct {
	Name           string
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
	// caps and sizes used across aggregator
	maxErrorSamples = 10
	maxLogEntries   = 500
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

// FileStat represents a path and its observed open count.
type FileStat struct {
	Path  string `json:"path"`
	Count int64  `json:"count"`
}

// SortField selects the column used when sorting Sorted output.
type SortField int

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

// CategoryStats holds per-category totals for the summary bar.
type CategoryStats struct {
	Count int64
	Errs  int64
}
