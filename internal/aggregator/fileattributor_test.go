package aggregator

import (
	"testing"
	"time"

	"github.com/fabianoflorentino/stracectl/internal/models"
)

func TestFileAttributor_OpenCloseAttribution(t *testing.T) {
	fa := NewDefaultFileAttributor()

	eOpen := models.SyscallEvent{Name: "open", Args: "\"/tmp/testfile\", O_RDONLY", RetVal: "4", PID: 1, Time: time.Now()}
	fa.AttributeFile(eOpen, "/tmp/testfile")

	// TopFiles should include the path
	tf := fa.TopFiles(0)
	found := false
	for _, f := range tf {
		if f.Path == "/tmp/testfile" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("TopFiles missing /tmp/testfile: %v", tf)
	}

	// TopFilesForSyscall for "open" should include the path
	tfs := fa.TopFilesForSyscall("open", 0)
	if len(tfs) == 0 || tfs[0].Path != "/tmp/testfile" {
		t.Fatalf("TopFilesForSyscall did not return /tmp/testfile: %v", tfs)
	}

	// Now close the fd and ensure fdmapper removes mapping and close attribution records
	eClose := models.SyscallEvent{Name: "close", Args: "4", PID: 1, Time: time.Now()}
	fa.HandleDupClose(eClose)

	// fd should be removed
	if p, ok := fa.(*defaultFileAttributor).fdmapper.Get(1, 4); ok && p != "" {
		t.Fatalf("fdmapper still had mapping after close: %v", p)
	}

	// Snapshot should show at least one entry for "close" (close attribution)
	snap := fa.Snapshot()
	if m, ok := snap["close"]; ok {
		if len(m) == 0 {
			t.Fatalf("expected close to have some entries in snapshot")
		}
	}
}
