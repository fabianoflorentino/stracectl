package cmd

import (
	"os"
	"testing"
)

func TestRunCmd_InvalidBackend(t *testing.T) {
	prev := runBackend
	defer func() { runBackend = prev }()
	runBackend = "invalid"
	if err := runCmd.RunE(nil, []string{"true"}); err == nil {
		t.Fatal("expected error for unknown backend")
	}
}

func TestTryElevateAndRerun_EnvSet(t *testing.T) {
	os.Setenv("STRACECTL_TRIED_ELEVATE", "1")
	defer os.Unsetenv("STRACECTL_TRIED_ELEVATE")
	// Should return early (no os.Exit) when env var is set.
	tryElevateAndRerun()
}
