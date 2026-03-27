package cmd

import (
	"os"
	"testing"
)

func TestTryElevateAndRerun_AlreadyTried(t *testing.T) {
	orig := os.Getenv("STRACECTL_TRIED_ELEVATE")
	t.Cleanup(func() { os.Setenv("STRACECTL_TRIED_ELEVATE", orig) })
	os.Setenv("STRACECTL_TRIED_ELEVATE", "1")

	// Should return quickly and not panic.
	tryElevateAndRerun()
}

func TestAttach_MissingArgs(t *testing.T) {
	if err := attachCmd.RunE(nil, []string{}); err == nil {
		t.Fatal("expected error when no PID or container provided")
	}
}
