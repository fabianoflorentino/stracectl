package aggregator

import (
	"maps"
	"sync"

	"github.com/fabianoflorentino/stracectl/internal/models"
)

// FileAttributor manages attribution of syscalls to file paths based on
// observed events and a mapping of (PID, FD) to paths.
type FileAttributor interface {
	AttributeFile(e models.SyscallEvent, path string)
	HandleFdBasedCall(e models.SyscallEvent)
	HandleDupClose(e models.SyscallEvent)
	TopFiles(n int) []FileStat
	TopFilesForSyscall(name string, n int) []FileStat
	Snapshot() map[string]map[string]int64
}

// defaultFileAttributor is a thread-safe implementation of FileAttributor that
// uses an FDMapper to track file descriptor mappings and maintains stats for file usage.
type defaultFileAttributor struct {
	mu              sync.RWMutex
	fileStats       map[string]int64
	fileStatsByCall map[string]map[string]int64
	fdmapper        FDMapper
}

// NewDefaultFileAttributor creates a new instance of defaultFileAttributor with initialized maps and a default FDMapper.
func NewDefaultFileAttributor() FileAttributor {
	return &defaultFileAttributor{
		fileStats:       make(map[string]int64),
		fileStatsByCall: make(map[string]map[string]int64),
		fdmapper:        NewDefaultFDMapper(),
	}
}

// AttributeFile processes an event that has an associated file path, updating stats and FD mappings as needed.
// For "open" and "openat" syscalls, it will also update the FDMapper with the new file descriptor returned by the syscall.
func (d *defaultFileAttributor) AttributeFile(e models.SyscallEvent, p string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.fileStats == nil {
		d.fileStats = make(map[string]int64)
	}
	if len(d.fileStats) < fileStatsCap || d.fileStats[p] > 0 {
		d.fileStats[p]++
	}
	if d.fileStatsByCall == nil {
		d.fileStatsByCall = make(map[string]map[string]int64)
	}
	if d.fileStatsByCall[e.Name] == nil {
		d.fileStatsByCall[e.Name] = make(map[string]int64)
	}
	if len(d.fileStatsByCall[e.Name]) < fileStatsCap || d.fileStatsByCall[e.Name][p] > 0 {
		d.fileStatsByCall[e.Name][p]++
	}

	if (e.Name == "open" || e.Name == "openat") && e.RetVal != "" {
		if fd, ok := parseRetInt(e.RetVal); ok && fd >= 0 {
			d.fdmapper.Set(e.PID, fd, p)
		}
	}
}

// HandleFdBasedCall processes syscalls that may be attributed to files based on
// their file descriptor arguments and the current FD mappings.For syscalls that
// take a file descriptor as an argument (e.g., "read", "write"), it looks up the
// path from the FDMapper and updates stats accordingly.
func (d *defaultFileAttributor) HandleFdBasedCall(e models.SyscallEvent) {
	if fd, ok := parseFirstIntArg(e.Args); ok {
		if path, ok2 := d.fdmapper.Get(e.PID, fd); ok2 && path != "" {
			d.mu.Lock()
			if d.fileStatsByCall == nil {
				d.fileStatsByCall = make(map[string]map[string]int64)
			}
			if d.fileStatsByCall[e.Name] == nil {
				d.fileStatsByCall[e.Name] = make(map[string]int64)
			}
			if len(d.fileStatsByCall[e.Name]) < fileStatsCap || d.fileStatsByCall[e.Name][path] > 0 {
				d.fileStatsByCall[e.Name][path]++
			}
			d.mu.Unlock()
		}
	}
}

// HandleDupClose processes "dup", "dup2", "dup3" and "close" syscalls to maintain the integrity of
// the FD mappings and file attribution stats. For "dup" syscalls, it updates the FDMapper to reflect
// the new file descriptor. For "close" syscalls, it removes the FD mapping and updates close attribution stats.
func (d *defaultFileAttributor) HandleDupClose(e models.SyscallEvent) {
	switch e.Name {
	case "dup", "dup2", "dup3":
		if oldfd, ok := parseFirstIntArg(e.Args); ok {
			if newfd, ok2 := parseRetInt(e.RetVal); ok2 && newfd >= 0 {
				if path, ok3 := d.fdmapper.Get(e.PID, oldfd); ok3 && path != "" {
					d.fdmapper.Set(e.PID, newfd, path)
				}
			}
		}
	case "close":
		if fd, ok := parseFirstIntArg(e.Args); ok {
			if path, ok2 := d.fdmapper.Get(e.PID, fd); ok2 && path != "" {
				d.mu.Lock()
				if d.fileStatsByCall == nil {
					d.fileStatsByCall = make(map[string]map[string]int64)
				}
				if d.fileStatsByCall[e.Name] == nil {
					d.fileStatsByCall[e.Name] = make(map[string]int64)
				}
				if len(d.fileStatsByCall[e.Name]) < fileStatsCap || d.fileStatsByCall[e.Name][path] > 0 {
					d.fileStatsByCall[e.Name][path]++
				}
				d.mu.Unlock()
			}
			d.fdmapper.Delete(e.PID, fd)
		}
	}
}

// TopFiles returns the top N files by total count across all syscalls. If n is 0, it returns all files.
// It uses the fileStats map to determine the counts for each file path.
func (d *defaultFileAttributor) TopFiles(n int) []FileStat {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return topFilesFromMap(d.fileStats, n)
}

// TopFilesForSyscall returns the top N files for a specific syscall name. If n is 0, it returns all files.
// It looks up the file stats for the given syscall name in the fileStatsByCall map.
func (d *defaultFileAttributor) TopFilesForSyscall(name string, n int) []FileStat {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return topFilesFromMap(d.fileStatsByCall[name], n)
}

// Snapshot returns a copy of the current file attribution stats organized by syscall name. The returned map is of the form:
// map[syscallName]map[filePath]count. This allows the caller to see how many times each file was involved in each syscall.
// The method creates deep copies of the internal maps to ensure thread safety and prevent external mutation of internal state.
func (d *defaultFileAttributor) Snapshot() map[string]map[string]int64 {
	d.mu.RLock()
	defer d.mu.RUnlock()

	out := make(map[string]map[string]int64, len(d.fileStatsByCall))
	for k, m := range d.fileStatsByCall {
		mm := make(map[string]int64, len(m))
		maps.Copy(mm, m)

		out[k] = mm
	}

	return out
}
