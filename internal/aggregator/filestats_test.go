package aggregator

import (
	"testing"
	"time"

	"github.com/fabianoflorentino/stracectl/internal/models"
)

func TestTopFiles_Counts(t *testing.T) {
	a := New()

	a.Add(models.SyscallEvent{Name: "open", Args: "\"/etc/hosts\", O_RDONLY", Time: time.Now()})
	a.Add(models.SyscallEvent{Name: "open", Args: "\"/etc/hosts\", O_RDONLY", Time: time.Now()})
	a.Add(models.SyscallEvent{Name: "openat", Args: "AT_FDCWD, \"/etc/ld.so.cache\", O_RDONLY", Time: time.Now()})
	a.Add(models.SyscallEvent{Name: "open", Args: "\"/tmp/foo\", O_RDONLY", Time: time.Now()})
	a.Add(models.SyscallEvent{Name: "openat", Args: "AT_FDCWD, \"/tmp/foo\", O_RDONLY", Time: time.Now()})

	files := a.TopFiles(0)
	if len(files) < 3 {
		t.Fatalf("TopFiles: want >=3 entries, got %d", len(files))
	}

	m := make(map[string]int64)
	for _, f := range files {
		m[f.Path] = f.Count
	}

	if m["/etc/hosts"] != 2 {
		t.Errorf("/etc/hosts count: want 2, got %d", m["/etc/hosts"])
	}
	if m["/tmp/foo"] != 2 {
		t.Errorf("/tmp/foo count: want 2, got %d", m["/tmp/foo"])
	}
	if m["/etc/ld.so.cache"] != 1 {
		t.Errorf("/etc/ld.so.cache count: want 1, got %d", m["/etc/ld.so.cache"])
	}
}

func TestTopFilesAndAttribution(t *testing.T) {
	a := New()
	a.Add(models.SyscallEvent{Name: "open", Args: "\"/tmp/foo\", O_RDONLY", RetVal: "3", PID: 1, Time: time.Now()})
	a.Add(models.SyscallEvent{Name: "close", Args: "3", RetVal: "", PID: 1, Time: time.Now()})
	files := a.TopFilesForSyscall("open", 0)
	if len(files) == 0 || files[0].Path != "/tmp/foo" {
		t.Fatalf("TopFilesForSyscall did not contain expected /tmp/foo; got %v", files)
	}
	top := a.TopFiles(0)
	found := false
	for _, f := range top {
		if f.Path == "/tmp/foo" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("TopFiles missing /tmp/foo")
	}
}
