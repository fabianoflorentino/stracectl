package aggregator

import (
	"os"
	"testing"

	"github.com/fabianoflorentino/stracectl/internal/procinfo"
)

func TestReadProcInfo_Self(t *testing.T) {
	pid := os.Getpid()
	info := procinfo.Read(pid)
	if info.PID != pid {
		t.Errorf("PID: want %d, got %d", pid, info.PID)
	}
	if info.Comm == "" {
		t.Error("Comm should be non-empty for the current process")
	}
	if info.Exe == "" {
		t.Error("Exe should be non-empty for the current process")
	}
}

func TestReadProcInfo_NonExistentPID(t *testing.T) {
	info := procinfo.Read(999999999)
	if info.PID != 999999999 {
		t.Errorf("PID should be set even when process doesn't exist; got %d", info.PID)
	}
}
