package procinfo

import (
	"os"
	"testing"
)

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
