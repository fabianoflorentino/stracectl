// Package aggregator accumulates SyscallEvent values and provides sorted views.
package aggregator

import (
	"maps"
	"sort"
	"sync"
	"time"

	"github.com/fabianoflorentino/stracectl/internal/models"
	"github.com/fabianoflorentino/stracectl/internal/procinfo"
)

// Aggregator is safe for concurrent use. Core responsibilities are limited to
// concurrency control and orchestration; helpers and types are defined in
// smaller files within the package to follow SRP.
type Aggregator struct {
	mu       sync.RWMutex
	stats    map[string]*SyscallStat
	total    int64
	errors   int64
	started  time.Time
	prevRate rateSnapshot
	rate     float64
	procInfo procinfo.ProcInfo
	logBuf   []LogEntry
	fileAttr FileAttributor
	done     bool
	nowFunc  func() time.Time
}

// rateSnapshot is a helper struct to track previous total and timestamp for rate calculations.
type rateSnapshot struct {
	total int64
	at    time.Time
}

// New creates and initializes a new Aggregator instance.
// The start time is set to the current time, and internal maps and slices are initialized.
func New() *Aggregator {
	now := time.Now()

	return &Aggregator{
		stats:    make(map[string]*SyscallStat),
		fileAttr: NewDefaultFileAttributor(),
		started:  now,
		prevRate: rateSnapshot{total: 0, at: now},
		nowFunc:  time.Now,
	}
}

// newWithClock returns an Aggregator configured to use the provided clock function.
// This helper is intended for tests where control over time progression is required.
func newWithClock(fn func() time.Time) *Aggregator {
	a := New()
	if fn != nil {
		a.nowFunc = fn
	}
	return a
}

// Add incorporates a new SyscallEvent into the aggregator, updating all relevant statistics,
// error counts, rates, and file attributions. It also appends the event to the live log buffer.
// This method is safe for concurrent use and handles all necessary locking internally.
func (a *Aggregator) Add(e models.SyscallEvent) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.total++

	// core stat updates
	a.addStatsLocked(e)

	// errors (if any)
	a.handleErrorLocked(e)

	// update rate (use a single now)
	now := a.nowFunc()
	a.updateRateLocked(now)

	// append to live log
	entry := LogEntry{
		Time:   e.Time,
		PID:    e.PID,
		Name:   e.Name,
		Args:   e.Args,
		RetVal: e.RetVal,
		Error:  e.Error,
	}
	a.appendLogLocked(entry)

	// file attribution and fd mapping (delegated)
	if p := extractPathFromArgs(e.Name, e.Args); p != "" {
		if len(p) > maxPathLen {
			p = p[:maxPathLen]
		}
		a.fileAttr.AttributeFile(e, p)
	} else {
		a.fileAttr.HandleFdBasedCall(e)
		a.fileAttr.HandleDupClose(e)
	}
}

// Sorted returns a slice of SyscallStat sorted by the specified field. It first obtains a snapshot
// of the current stats and file attributions under lock, then releases the lock to perform sorting
// and percentile calculations on the snapshot copy. This minimizes lock contention while still providing a consistent view of the data at the time of the call.
func (a *Aggregator) Sorted(by SortField) []SyscallStat {
	a.mu.RLock()
	statsCopy, fileMapSnap := a.snapshotLocked()
	a.mu.RUnlock()

	out := a.finalizeSnapshot(statsCopy, fileMapSnap, time.Now().Unix())

	sort.Slice(out, func(i, j int) bool {
		switch by {
		case SortByTotal:
			return out[i].TotalTime > out[j].TotalTime
		case SortByAvg:
			return out[i].AvgTime() > out[j].AvgTime()
		case SortByMin:
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
		default:
			return out[i].Count > out[j].Count
		}
	})

	return out
}

// Get returns the SyscallStat for the specified syscall name, if it exists.
// It computes percentiles and error rates on the fly before returning the stat.
// This method is safe for concurrent use and handles all necessary locking internally.
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
	cp.ErrRate60s = s.errWin.sum(a.nowFunc().Unix())

	return cp, true
}

// CategoryBreakdown returns a map of syscall categories to their aggregated counts and error counts.
// This method is safe for concurrent use and handles all necessary locking internally.
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

// Total returns the total number of syscall events recorded by the aggregator.
// This method is safe for concurrent use and handles all necessary locking internally.
func (a *Aggregator) Total() int64 {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.total
}

// Errors returns the total number of syscall events that resulted in an error.
// This method is safe for concurrent use and handles all necessary locking internally.
func (a *Aggregator) Errors() int64 {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.errors
}

// UniqueCount returns the number of unique syscalls recorded by the aggregator.
// This method is safe for concurrent use and handles all necessary locking internally.
func (a *Aggregator) UniqueCount() int {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return len(a.stats)
}

// Rate returns the recent syscalls-per-second rate.
// This method is safe for concurrent use and handles all necessary locking internally.
func (a *Aggregator) Rate() float64 {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.rate
}

// StartTime returns the time the aggregator was created.
// This method is safe for concurrent use and handles all necessary locking internally.
func (a *Aggregator) StartTime() time.Time {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.started
}

// SetProcInfo stores process metadata for the traced process.
// This method is safe for concurrent use and handles all necessary locking internally.
func (a *Aggregator) SetProcInfo(info procinfo.ProcInfo) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.procInfo = info
}

// GetProcInfo returns the stored process metadata.
// This method is safe for concurrent use and handles all necessary locking internally.
func (a *Aggregator) GetProcInfo() procinfo.ProcInfo {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.procInfo
}

// RecentLog returns a copy of the live-log ring buffer (oldest first).
// This method is safe for concurrent use and handles all necessary locking internally.
func (a *Aggregator) RecentLog() []LogEntry {
	a.mu.RLock()
	defer a.mu.RUnlock()

	out := make([]LogEntry, len(a.logBuf))
	copy(out, a.logBuf)

	return out
}

// SetDone marks the traced process as having exited.
// This method is safe for concurrent use and handles all necessary locking internally.
func (a *Aggregator) SetDone() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.done = true
}

// IsDone reports whether the traced process has exited.
// This method is safe for concurrent use and handles all necessary locking internally.
func (a *Aggregator) IsDone() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.done
}

// TopFiles returns the most commonly accessed files across all syscalls, sorted by access count.
// This method is safe for concurrent use and handles all necessary locking internally.
func (a *Aggregator) TopFiles(n int) []FileStat {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.fileAttr.TopFiles(n)
}

// TopFilesForSyscall returns the most commonly accessed files for a specific syscall, sorted by access count.
// This method is safe for concurrent use and handles all necessary locking internally.
func (a *Aggregator) TopFilesForSyscall(name string, n int) []FileStat {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.fileAttr.TopFilesForSyscall(name, n)
}

// -- private helper functions (assume lock is held) -----------------------

// addStatsLocked updates the syscall stat for the event's syscall name, incrementing counts and updating latency histograms.
// This method assumes the caller holds `a.mu` (Lock). It does not perform any locking itself.
func (a *Aggregator) addStatsLocked(e models.SyscallEvent) {
	s := a.getOrCreateStatLocked(e.Name)
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
}

// handleErrorLocked updates the error stats for the event's syscall name.
// This method assumes the caller holds `a.mu` (Lock). It does not perform any locking itself.
func (a *Aggregator) handleErrorLocked(e models.SyscallEvent) {
	if !e.IsError() {
		return
	}

	s := a.getOrCreateStatLocked(e.Name)
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
		sec = a.nowFunc().Unix()
	}

	s.errWin.record(sec)
	// ring buffer: keep the most recent maxErrorSamples samples
	sample := ErrorSample{
		Args:  e.Args,
		Errno: e.Error,
		Time:  e.Time,
	}

	if len(s.RecentErrors) < maxErrorSamples {
		s.RecentErrors = append(s.RecentErrors, sample)
	} else {
		copy(s.RecentErrors, s.RecentErrors[1:])
		s.RecentErrors[maxErrorSamples-1] = sample
	}
}

// updateRateLocked updates the recent syscalls-per-second rate if at least 500ms have passed since the last update.
// This method assumes the caller holds `a.mu` (Lock). It does not perform any locking itself.
func (a *Aggregator) updateRateLocked(now time.Time) {
	if now.Sub(a.prevRate.at) >= 500*time.Millisecond {
		dt := now.Sub(a.prevRate.at).Seconds()
		if dt > 0 {
			a.rate = float64(a.total-a.prevRate.total) / dt
		}

		a.prevRate = rateSnapshot{total: a.total, at: now}
	}
}

// appendLogLocked appends a log entry to the log buffer.
// This method assumes the caller holds `a.mu` (Lock). It does not perform any locking itself.
func (a *Aggregator) appendLogLocked(entry LogEntry) {
	if len(a.logBuf) < maxLogEntries {
		a.logBuf = append(a.logBuf, entry)
	} else {
		copy(a.logBuf, a.logBuf[1:])
		a.logBuf[maxLogEntries-1] = entry
	}
}

// getOrCreateStatLocked retrieves the SyscallStat for the given syscall name, creating it if it does not exist.
// This method assumes the caller holds `a.mu` (Lock). It does not perform any locking itself.
func (a *Aggregator) getOrCreateStatLocked(name string) *SyscallStat {
	s := a.stats[name]

	if s == nil {
		s = &SyscallStat{Name: name, Category: classify(name)}
		a.stats[name] = s
	}

	return s
}

// snapshotLocked makes deep-ish copies of current stats and file maps.
// It assumes the caller holds `a.mu` (RLock or Lock).
func (a *Aggregator) snapshotLocked() ([]SyscallStat, map[string]map[string]int64) {
	statsCopy := make([]SyscallStat, 0, len(a.stats))
	// obtain a snapshot of fileStatsByCall from the file attributor
	fileMapSnap := a.fileAttr.Snapshot()
	for _, s := range a.stats {
		cp := *s

		if s.ErrorBreakdown != nil {
			cp.ErrorBreakdown = make(map[string]int64, len(s.ErrorBreakdown))
			maps.Copy(cp.ErrorBreakdown, s.ErrorBreakdown)
		}
		if len(s.RecentErrors) > 0 {
			cp.RecentErrors = make([]ErrorSample, len(s.RecentErrors))
			copy(cp.RecentErrors, s.RecentErrors)
		}

		statsCopy = append(statsCopy, cp)
	}

	return statsCopy, fileMapSnap
}

// finalizeSnapshot computes percentiles, error rates and picks best file
// for each syscall snapshot. This does not require locks because it works
// on copies produced by `snapshotLocked`.
func (a *Aggregator) finalizeSnapshot(statsCopy []SyscallStat, fileMapSnap map[string]map[string]int64, now int64) []SyscallStat {
	out := make([]SyscallStat, 0, len(statsCopy))

	for _, cp := range statsCopy {
		cp.P95 = latPercentile(&cp.latHist, 95)
		cp.P99 = latPercentile(&cp.latHist, 99)
		cp.ErrRate60s = cp.errWin.sum(now)

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

	return out
}
