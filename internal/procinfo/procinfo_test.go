package procinfo

import (
	"os"
	"testing"
)

// TestRead_CurrentProcess verifies that ProcInfo.Read can successfully read
// metadata for the current process. It checks that the PID matches and that at
// least one of the fields is non-empty, which indicates that the reading logic
// is functioning (even if some fields may be inaccessible in certain environments).
func TestRead_CurrentProcess(t *testing.T) {
	pid := os.Getpid()
	info := Read(pid)

	if info.PID != pid {
		t.Fatalf("expected PID %d, got %d", pid, info.PID)
	}

	if info.Cmdline == "" && info.Comm == "" && info.Exe == "" && info.Cwd == "" {
		t.Fatalf("expected at least one proc field to be non-empty")
	}
}
