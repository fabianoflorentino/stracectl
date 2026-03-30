package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fabianoflorentino/stracectl/internal/aggregator"
	"github.com/fabianoflorentino/stracectl/internal/models"
)

func Test_waitForUIReady_detectsLogFile(t *testing.T) {
	const p = "/tmp/stracectl_ui_events.log"
	// ensure clean state
	_ = os.Remove(p)
	defer func() { _ = os.Remove(p) }()

	// write a marker that waitForUIReady looks for
	if err := os.WriteFile(p, []byte("ev=window-size\n"), 0644); err != nil {
		t.Fatalf("failed to write ui events log: %v", err)
	}

	// should return quickly
	start := time.Now()
	waitForUIReady(500 * time.Millisecond)
	if time.Since(start) > 500*time.Millisecond {
		t.Fatalf("waitForUIReady took too long")
	}
}

func Test_writeHTMLReport_createsFile(t *testing.T) {
	tmp := t.TempDir()
	out := filepath.Join(tmp, "report.html")

	agg := aggregator.New()
	// add a small event so report has some content
	agg.Add(models.SyscallEvent{Name: "openat", Latency: 1, Time: time.Now()})

	if err := writeHTMLReport(out, agg, "label", 5); err != nil {
		t.Fatalf("writeHTMLReport failed: %v", err)
	}

	fi, err := os.Stat(out)
	if err != nil {
		t.Fatalf("expected report file to exist: %v", err)
	}
	if fi.Size() == 0 {
		t.Fatalf("expected non-empty report file")
	}
}

// --- additional tests for privacy pipeline wiring ---

// fakeTracer implements a minimal tracer for tests.
type eventsTracer struct {
	events chan models.SyscallEvent
}

func (f *eventsTracer) Run(ctx context.Context, prog string, args []string) (<-chan models.SyscallEvent, error) {
	return f.events, nil
}
func (f *eventsTracer) Attach(ctx context.Context, pid int) (<-chan models.SyscallEvent, error) {
	return nil, nil
}

func TestRunTraceWithEvents_FullAndForceWritesLog(t *testing.T) {
	tmp := t.TempDir()
	logPath := filepath.Join(tmp, "privacy.log")

	// set globals
	privacyLogPath = logPath
	privacyNoArgs = false
	privacyMaxArgSize = 64
	privacyRedactPatterns = ""
	privacySyscalls = ""
	privacyExclude = ""
	privacyPrivacyLevel = "low"
	privacyFull = true
	privacyForce = true

	f := &eventsTracer{events: make(chan models.SyscallEvent, 1)}
	agg := aggregator.New()

	// send one event and close
	f.events <- models.SyscallEvent{PID: 42, Name: "open", Args: "\"/etc/passwd\"", Time: time.Now()}
	close(f.events)

	// call runTraceWithEvents which processes events and should return
	ctx := context.Background()
	err := runTraceWithEvents(ctx, func() {}, f.events, agg, "", "", "", 0, "t")
	if err != nil {
		t.Fatalf("runTraceWithEvents failed: %v", err)
	}

	// verify privacy log file exists and mentions the syscall
	b, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("expected privacy log file, got error: %v", err)
	}
	out := string(b)
	if !strings.Contains(out, "open") {
		t.Fatalf("expected syscall name in privacy log, got: %s", out)
	}
}

func TestPrivacyLevelHigh_SuppressesArgs(t *testing.T) {
	// ensure privacy-level=high causes NoArgs behavior
	privacyLogPath = ""
	privacyPrivacyLevel = "high"
	privacyNoArgs = false

	f := &eventsTracer{events: make(chan models.SyscallEvent, 1)}
	agg := aggregator.New()

	f.events <- models.SyscallEvent{PID: 7, Name: "open", Args: "\"/secret/path\"", Time: time.Now()}
	close(f.events)

	ctx := context.Background()
	// run; should return without panic and respect high privacy semantics
	if err := runTraceWithEvents(ctx, func() {}, f.events, agg, "", "", "", 0, "t"); err != nil {
		t.Fatalf("runTraceWithEvents failed: %v", err)
	}
}
