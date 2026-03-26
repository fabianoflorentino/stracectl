package aggregator

import "sync"

// FDMapper manages the mapping of (PID, FD) to file paths for attribution purposes.
type FDMapper interface {
	Set(pid, fd int, path string)
	Get(pid, fd int) (string, bool)
	Delete(pid, fd int)
}

// defaultFDMapper is a thread-safe implementation of FDMapper using a nested map.
type defaultFDMapper struct {
	mu sync.RWMutex
	m  map[int]map[int]string
}

// NewDefaultFDMapper creates a new instance of defaultFDMapper.
func NewDefaultFDMapper() FDMapper {
	return &defaultFDMapper{m: make(map[int]map[int]string)}
}

// Set associates a (PID, FD) pair with a file path.
// This is typically called when an "open" or "openat" syscall returns a new file descriptor.
func (d *defaultFDMapper) Set(pid, fd int, path string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.m[pid] == nil {
		d.m[pid] = make(map[int]string)
	}

	d.m[pid][fd] = path
}

// Get retrieves the file path associated with a (PID, FD) pair.
// It returns the path and a boolean indicating whether the mapping was found.
func (d *defaultFDMapper) Get(pid, fd int) (string, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if m := d.m[pid]; m != nil {
		p, ok := m[fd]
		return p, ok
	}

	return "", false
}

// Delete removes the mapping for a (PID, FD) pair.
// This is typically called when a "close" syscall is observed for the file descriptor.
func (d *defaultFDMapper) Delete(pid, fd int) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if m := d.m[pid]; m != nil {
		delete(m, fd)
	}
}
