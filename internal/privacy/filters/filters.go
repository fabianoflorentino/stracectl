package filters

import (
	"strings"

	"github.com/fabianoflorentino/stracectl/internal/privacy"
)

// FilterSet implements a simple include/exclude filter for syscalls and scopes.
type FilterSet struct {
	include map[string]bool
	exclude map[string]bool
	pids    map[int]bool
	uids    map[int]bool
}

// New creates a FilterSet. Pass comma-separated syscall lists (can be empty).
func New(includeList, excludeList string, pids []int, uids []int) *FilterSet {
	f := &FilterSet{
		include: make(map[string]bool),
		exclude: make(map[string]bool),
		pids:    make(map[int]bool),
		uids:    make(map[int]bool),
	}

	for _, s := range strings.Split(includeList, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		f.include[s] = true
	}
	for _, s := range strings.Split(excludeList, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		f.exclude[s] = true
	}
	for _, id := range pids {
		f.pids[id] = true
	}
	for _, id := range uids {
		f.uids[id] = true
	}

	return f
}

// Allow returns true if the event passes the filter.
func (f *FilterSet) Allow(e *privacy.TraceEvent) bool {
	// PID/user filtering: if PID list is non-empty and PID not in list => reject
	if len(f.pids) > 0 {
		if !f.pids[e.PID] {
			return false
		}
	}
	if len(f.uids) > 0 {
		if !f.uids[e.UID] {
			return false
		}
	}

	// Exclude has priority — if syscall is in exclude -> reject
	if e.Syscall != "" && f.exclude[e.Syscall] {
		return false
	}

	// If include list is non-empty, only allow if present
	if len(f.include) > 0 {
		if e.Syscall == "" {
			return false
		}
		if !f.include[e.Syscall] {
			return false
		}
	}

	return true
}
