// Package aggregator accumulates SyscallEvent values and provides sorted views.
package aggregator

import (
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
	mu        sync.RWMutex
	stats     map[string]*SyscallStat
	total     int64
	errors    int64
	started   time.Time
	prevRate  rateSnapshot
	rate      float64
	procInfo  procinfo.ProcInfo
	logBuf    []LogEntry
	fileStats map[string]int64
	// fileStatsByCall maps syscall name -> (path -> count)
	fileStatsByCall map[string]map[string]int64
	// fdToPath maps pid -> fd -> path, used to attribute fd-based syscalls to paths
	fdToPath map[int]map[int]string
	done     bool
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
		prevRate:        rateSnapshot{total: 0, at: now},
	}
}

// Add records one event.
func (a *Aggregator) Add(e models.SyscallEvent) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.total++

	// core stat updates
	a.addStatsLocked(e)

	// errors (if any)
	a.handleErrorLocked(e)

	// update rate (use a single now)
	now := time.Now()
	a.updateRateLocked(now)

	// append to live log
	entry := LogEntry{Time: e.Time, PID: e.PID, Name: e.Name, Args: e.Args, RetVal: e.RetVal, Error: e.Error}
	a.appendLogLocked(entry)

	// file attribution and fd mapping
	if p := extractPathFromArgs(e.Name, e.Args); p != "" {
		if len(p) > maxPathLen {
			p = p[:maxPathLen]
		}
		a.attributeFileLocked(e, p)
	} else {
		a.handleFdBasedCallLocked(e)
		a.handleDupCloseLocked(e)
	}
}

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

func (a *Aggregator) TopFiles(n int) []FileStat {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return topFilesFromMap(a.fileStats, n)
}

func (a *Aggregator) TopFilesForSyscall(name string, n int) []FileStat {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return topFilesFromMap(a.fileStatsByCall[name], n)
}

// -- private helper functions (assume lock is held) -----------------------

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
		sec = time.Now().Unix()
	}
	s.errWin.record(sec)
	// ring buffer: keep the most recent maxErrorSamples samples
	sample := ErrorSample{Args: e.Args, Errno: e.Error, Time: e.Time}
	if len(s.RecentErrors) < maxErrorSamples {
		s.RecentErrors = append(s.RecentErrors, sample)
	} else {
		copy(s.RecentErrors, s.RecentErrors[1:])
		s.RecentErrors[maxErrorSamples-1] = sample
	}
}

func (a *Aggregator) updateRateLocked(now time.Time) {
	if now.Sub(a.prevRate.at) >= 500*time.Millisecond {
		dt := now.Sub(a.prevRate.at).Seconds()
		if dt > 0 {
			a.rate = float64(a.total-a.prevRate.total) / dt
		}
		a.prevRate = rateSnapshot{total: a.total, at: now}
	}
}

func (a *Aggregator) appendLogLocked(entry LogEntry) {
	if len(a.logBuf) < maxLogEntries {
		a.logBuf = append(a.logBuf, entry)
	} else {
		copy(a.logBuf, a.logBuf[1:])
		a.logBuf[maxLogEntries-1] = entry
	}
}

func (a *Aggregator) attributeFileLocked(e models.SyscallEvent, p string) {
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
}

func (a *Aggregator) handleFdBasedCallLocked(e models.SyscallEvent) {
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
}

func (a *Aggregator) handleDupCloseLocked(e models.SyscallEvent) {
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
					delete(m, fd)
				}
			}
		}
	}
}

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
	fileMapSnap := make(map[string]map[string]int64, len(a.fileStatsByCall))
	for name, s := range a.stats {
		cp := *s
		if s.ErrorBreakdown != nil {
			cp.ErrorBreakdown = make(map[string]int64, len(s.ErrorBreakdown))
			for k, v := range s.ErrorBreakdown {
				cp.ErrorBreakdown[k] = v
			}
		}
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
