package models

import "time"

// SyscallEvent represents a single syscall captured from a traced process.
type SyscallEvent struct {
	PID     int
	Name    string
	Args    string
	RetVal  string
	Error   string        // POSIX error name, e.g. "ENOENT"
	Latency time.Duration // time spent in kernel, from strace -T
	Time    time.Time
}

// IsError reports whether the syscall returned an error.
func (e SyscallEvent) IsError() bool {
	return e.Error != ""
}
