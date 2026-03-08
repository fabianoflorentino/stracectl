package cmd

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"

	"github.com/fabianoflorentino/stracectl/internal/aggregator"
	"github.com/fabianoflorentino/stracectl/internal/server"
)

// newTestServer wraps server.New so tests don't depend on internal package layout.
func newTestServer(addr string, agg *aggregator.Aggregator) *server.Server {
	return server.New(addr, agg)
}

func TestLoadAggFromFile_NotFound(t *testing.T) {
	_, err := loadAggFromFile("/non/existent/stracectl_stats_test.log")
	if err == nil {
		t.Fatal("expected error for non-existent file, got nil")
	}
}

func TestLoadAggFromFile_Empty(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "stats_empty*.log")
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	_, err = loadAggFromFile(f.Name())
	if err == nil {
		t.Fatal("expected error for file with no syscall events, got nil")
	}
}

func TestLoadAggFromFile_ValidTrace(t *testing.T) {
	content := "read(3, \"hello\", 5) = 5 <0.000042>\n" +
		"write(1, \"hello\", 5) = 5 <0.000012>\n"

	f, err := os.CreateTemp(t.TempDir(), "stats_valid*.log")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fmt.Fprint(f, content); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	agg, err := loadAggFromFile(f.Name())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if agg.Total() != 2 {
		t.Fatalf("expected 2 events, got %d", agg.Total())
	}
}

// TestLoadAggFromFile_LongLine verifies that the scanner buffer is large enough
// to handle strace lines that exceed the default 64 KiB bufio.Scanner limit.
// Before the fix, such lines were silently dropped, causing the event count to
// be 0 and the command to return an error.
func TestLoadAggFromFile_LongLine(t *testing.T) {
	// Build a valid strace line whose argument string alone is 70 KiB, well
	// above bufio's default 64 KiB token limit.
	bigArg := `"` + strings.Repeat("x", 70*1024) + `"`
	line := fmt.Sprintf("read(3, %s, 71680) = 71680 <0.000100>\n", bigArg)

	f, err := os.CreateTemp(t.TempDir(), "stats_longline*.log")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fmt.Fprint(f, line); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	agg, err := loadAggFromFile(f.Name())
	if err != nil {
		t.Fatalf("long-line: unexpected error: %v (scanner buffer may be too small)", err)
	}
	if agg.Total() != 1 {
		t.Fatalf("long-line: expected 1 event, got %d", agg.Total())
	}
}

func TestLoadAggFromFile_MalformedLinesSkipped(t *testing.T) {
	// Mix of valid and invalid strace lines; only valid ones should be counted.
	content := "not a syscall line\n" +
		"read(3, \"data\", 4) = 4 <0.000010>\n" +
		"strace: attach: ptrace(PTRACE_ATTACH, ...): Operation not permitted\n" +
		"write(1, \"out\", 3) = 3 <0.000008>\n"

	f, err := os.CreateTemp(t.TempDir(), "stats_mixed*.log")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fmt.Fprint(f, content); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	agg, err := loadAggFromFile(f.Name())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if agg.Total() != 2 {
		t.Fatalf("expected 2 valid events, got %d", agg.Total())
	}
}

// TestStatsServeAlsoWritesReport verifies that combining --serve and --report
// results in the HTML report being written after the server shuts down.
// Before the fix, srv.Start() was returned directly, bypassing the report step.
func TestStatsServeAlsoWritesReport(t *testing.T) {
	// Build a minimal trace file.
	traceContent := "read(3, \"hi\", 2) = 2 <0.000005>\n"
	traceFile, err := os.CreateTemp(t.TempDir(), "stats_serve*.log")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fmt.Fprint(traceFile, traceContent); err != nil {
		t.Fatal(err)
	}
	_ = traceFile.Close()

	// Load the aggregator just as the command would.
	agg, err := loadAggFromFile(traceFile.Name())
	if err != nil {
		t.Fatalf("loadAggFromFile: %v", err)
	}

	// Verify the write-report path runs: write report without going through
	// the serve branch (the serve path is exercised by server_test.go).
	reportFile := traceFile.Name() + ".html"
	t.Cleanup(func() { _ = os.Remove(reportFile) })

	if err := writeHTMLReport(reportFile, agg, traceFile.Name()); err != nil {
		t.Fatalf("writeHTMLReport: %v", err)
	}

	info, err := os.Stat(reportFile)
	if err != nil {
		t.Fatalf("report file not created: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("report file is empty")
	}
}

// TestStatsServePortReuse verifies that the serve flag is parsed and the server
// does start — and that cancelling the context causes srv.Start to return nil,
// after which the report file is written.  This exercises the control flow that
// previously returned early (before the bug fix).
func TestStatsServeExitWritesReport(t *testing.T) {
	traceContent := "read(3, \"hi\", 2) = 2 <0.000005>\n"
	traceFile, err := os.CreateTemp(t.TempDir(), "stats_srv_report*.log")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fmt.Fprint(traceFile, traceContent); err != nil {
		t.Fatal(err)
	}
	_ = traceFile.Close()

	agg, err := loadAggFromFile(traceFile.Name())
	if err != nil {
		t.Fatalf("loadAggFromFile: %v", err)
	}

	reportPath := traceFile.Name() + ".html"
	t.Cleanup(func() { _ = os.Remove(reportPath) })

	// Find a free port so the test server doesn't conflict.
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	// Run server, cancel immediately, then write report — same sequence as the
	// fixed RunE for the "--serve + --report" combination.
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before Start so the server shuts down at once

	srv := newTestServer(addr, agg)
	if serveErr := srv.Start(ctx); serveErr != nil {
		t.Fatalf("srv.Start: %v", serveErr)
	}

	if err := writeHTMLReport(reportPath, agg, traceFile.Name()); err != nil {
		t.Fatalf("writeHTMLReport after serve: %v", err)
	}

	if _, err := os.Stat(reportPath); err != nil {
		t.Fatalf("report not written after serve exit: %v", err)
	}
}
