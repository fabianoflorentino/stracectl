package aggregator

import (
	"maps"
	"sync"

	"github.com/fabianoflorentino/stracectl/internal/models"
)

// FileAttributor handles attribution of syscalls to file paths.
type FileAttributor interface {
	AttributeFile(e models.SyscallEvent, path string)
	HandleFdBasedCall(e models.SyscallEvent)
	HandleDupClose(e models.SyscallEvent)
	TopFiles(n int) []FileStat
	TopFilesForSyscall(name string, n int) []FileStat
	Snapshot() map[string]map[string]int64
}

type defaultFileAttributor struct {
	mu              sync.RWMutex
	fileStats       map[string]int64
	fileStatsByCall map[string]map[string]int64
	fdmapper        FDMapper
}

func NewDefaultFileAttributor() FileAttributor {
	return &defaultFileAttributor{
		fileStats:       make(map[string]int64),
		fileStatsByCall: make(map[string]map[string]int64),
		fdmapper:        NewDefaultFDMapper(),
	}
}

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

func (d *defaultFileAttributor) TopFiles(n int) []FileStat {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return topFilesFromMap(d.fileStats, n)
}

func (d *defaultFileAttributor) TopFilesForSyscall(name string, n int) []FileStat {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return topFilesFromMap(d.fileStatsByCall[name], n)
}

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
