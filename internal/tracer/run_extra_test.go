package tracer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestRun_FakeStrace uses a temporary "strace" wrapper that execs the
// current test binary with GO_FAKE_STRACE set, allowing exercising the
// Run() code path (SysProcAttr/Setpgid and Cancel wiring) without requiring
// the real strace binary to be installed.
func TestRun_FakeStrace(t *testing.T) {
	exe, err := os.Executable()
	t.Helper()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}

	tmp := t.TempDir()
	script := fmt.Sprintf("#!/bin/sh\nexec %s -test.run=^$ \"$@\"\n", exe)
	path := filepath.Join(tmp, "strace")
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	orig := os.Getenv("PATH")
	t.Cleanup(func() { os.Setenv("PATH", orig); os.Unsetenv("GO_FAKE_STRACE") })
	os.Setenv("PATH", tmp+string(os.PathListSeparator)+orig)
	os.Setenv("GO_FAKE_STRACE", "lines")

	tr := NewStraceTracer()
	ctx, cancel := context.WithCancel(context.Background())
	ch, err := tr.Run(ctx, "ignored", nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Cancel to exercise cancellation / Wait-paths and ensure channel closes.
	cancel()

	select {
	case <-drain(ch):
		// success
	case <-time.After(5 * time.Second):
		t.Fatal("channel did not close after cancel")
	}
}
