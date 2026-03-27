package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

// TestTryElevate_EarlyReturn ensures the function exits early when the
// STRACECTL_TRIED_ELEVATE environment variable is set.
func TestTryElevate_EarlyReturn(t *testing.T) {
	orig := os.Getenv("STRACECTL_TRIED_ELEVATE")
	t.Cleanup(func() { os.Setenv("STRACECTL_TRIED_ELEVATE", orig) })
	os.Setenv("STRACECTL_TRIED_ELEVATE", "1")

	// Should simply return and not panic or exit.
	tryElevateAndRerun()
}

// TestTryElevate_FailingCommand ensures that when the invoked elevation
// helper (sudo/prlimit) fails, the function returns without exiting.
func TestTryElevate_FailingCommand(t *testing.T) {
	tmp := t.TempDir()
	// create fake sudo and prlimit that exit non-zero
	for _, name := range []string{"sudo", "prlimit"} {
		path := filepath.Join(tmp, name)
		if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 2\n"), 0755); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	origPath := os.Getenv("PATH")
	t.Cleanup(func() { os.Setenv("PATH", origPath); os.Unsetenv("STRACECTL_TRIED_ELEVATE") })
	os.Setenv("PATH", tmp+string(os.PathListSeparator)+origPath)
	os.Unsetenv("STRACECTL_TRIED_ELEVATE")

	// set args to include something to run; doesn't matter because helper fails
	origArgs := os.Args
	t.Cleanup(func() { os.Args = origArgs })
	os.Args = append([]string{origArgs[0]}, "run", "true")

	tryElevateAndRerun()
}
