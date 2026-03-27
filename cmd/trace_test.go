package cmd

import (
	"os"
	"path/filepath"
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
