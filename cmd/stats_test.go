package cmd

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

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
