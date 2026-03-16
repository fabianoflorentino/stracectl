package cmd

import (
	"context"
	"os"
	"testing"

	"github.com/fabianoflorentino/stracectl/internal/models"
)

// noopTracer implements tracer.Tracer but not the optional config setters.
type noopTracer struct{}

func (n *noopTracer) Attach(ctx context.Context, pid int) (<-chan models.SyscallEvent, error) {
	return nil, nil
}
func (n *noopTracer) Run(ctx context.Context, program string, args []string) (<-chan models.SyscallEvent, error) {
	return nil, nil
}

func TestRunCmd_InvalidBackend(t *testing.T) {
	prev := runBackend
	defer func() { runBackend = prev }()
	runBackend = "invalid"
	if err := runCmd.RunE(nil, []string{"true"}); err == nil {
		t.Fatal("expected error for unknown backend")
	}
}

func TestAttach_InvalidPID(t *testing.T) {
	if err := attachCmd.RunE(nil, []string{"not-a-number"}); err == nil {
		t.Fatal("expected parse error for PID")
	}
}

func TestApplyEBPFOptions_NoConfig(t *testing.T) {
	// Should be a no-op and not panic when tracer does not implement setters.
	applyEBPFOptions(&noopTracer{}, true, true)
}

func TestTryElevateAndRerun_EnvSet(t *testing.T) {
	os.Setenv("STRACECTL_TRIED_ELEVATE", "1")
	defer os.Unsetenv("STRACECTL_TRIED_ELEVATE")
	// Should return early (no os.Exit) when env var is set.
	tryElevateAndRerun()
}
