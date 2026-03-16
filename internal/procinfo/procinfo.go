package procinfo

import (
	"fmt"
	"os"
	"strings"
)

// ProcInfo holds process metadata read from /proc/<pid>.
// Fields are exported for JSON rendering by consumers.
type ProcInfo struct {
	PID     int
	Comm    string // short name from /proc/<pid>/comm
	Cmdline string // full command line from /proc/<pid>/cmdline
	Exe     string // executable path via /proc/<pid>/exe symlink
	Cwd     string // working directory via /proc/<pid>/cwd symlink
}

// Read reads process metadata from /proc/<pid>.
// Missing or inaccessible fields are silently omitted (empty string).
func Read(pid int) ProcInfo {
	base := fmt.Sprintf("/proc/%d", pid)
	info := ProcInfo{PID: pid}
	// The paths are constructed from a numeric PID so there is no traversal risk.
	// G304 is intentionally not flagged here by callers that use it.
	if b, err := os.ReadFile(base + "/comm"); err == nil { //nolint:gosec // path is /proc/<pid>/comm (numeric PID)
		info.Comm = strings.TrimSpace(string(b))
	}
	if b, err := os.ReadFile(base + "/cmdline"); err == nil { //nolint:gosec // path is /proc/<pid>/cmdline (numeric PID)
		// cmdline is NUL-separated; convert to space-separated and trim trailing NUL
		info.Cmdline = strings.TrimRight(strings.ReplaceAll(string(b), "\x00", " "), " ")
	}
	if exe, err := os.Readlink(base + "/exe"); err == nil {
		info.Exe = exe
	}
	if cwd, err := os.Readlink(base + "/cwd"); err == nil {
		info.Cwd = cwd
	}
	return info
}
