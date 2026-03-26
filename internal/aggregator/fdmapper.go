package aggregator

import "sync"

// FDMapper manages pid->fd->path mappings.
type FDMapper interface {
	Set(pid, fd int, path string)
	Get(pid, fd int) (string, bool)
	Delete(pid, fd int)
}

type defaultFDMapper struct {
	mu sync.RWMutex
	m  map[int]map[int]string
}

func NewDefaultFDMapper() FDMapper {
	return &defaultFDMapper{m: make(map[int]map[int]string)}
}

func (d *defaultFDMapper) Set(pid, fd int, path string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.m[pid] == nil {
		d.m[pid] = make(map[int]string)
	}
	d.m[pid][fd] = path
}

func (d *defaultFDMapper) Get(pid, fd int) (string, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if m := d.m[pid]; m != nil {
		p, ok := m[fd]
		return p, ok
	}
	return "", false
}

func (d *defaultFDMapper) Delete(pid, fd int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if m := d.m[pid]; m != nil {
		delete(m, fd)
	}
}
