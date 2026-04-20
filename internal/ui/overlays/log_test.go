package overlays

import (
	"strings"
	"testing"
	"time"

	"github.com/fabianoflorentino/stracectl/internal/aggregator"
	"github.com/fabianoflorentino/stracectl/internal/procinfo"
)

// mockAgg implements a minimal AggregatorView used by the tests in this file.
type mockAgg struct {
	logs []aggregator.LogEntry
}

func (m mockAgg) Total() int64                                                        { return 0 }
func (m mockAgg) Errors() int64                                                       { return 0 }
func (m mockAgg) Rate() float64                                                       { return 0 }
func (m mockAgg) UniqueCount() int                                                    { return 0 }
func (m mockAgg) Sorted(aggregator.SortField) []aggregator.SyscallStat                { return nil }
func (m mockAgg) RecentLog() []aggregator.LogEntry                                    { return m.logs }
func (m mockAgg) TopFiles(n int) []aggregator.FileStat                                { return nil }
func (m mockAgg) CategoryBreakdown() map[aggregator.Category]aggregator.CategoryStats { return nil }
func (m mockAgg) GetProcInfo() procinfo.ProcInfo                                      { return procinfo.ProcInfo{} }
func (m mockAgg) IsPerPID() bool                                                      { return false }

// TestStringsRepeat verifies the behavior of the stringsRepeat helper function.
// This is a simple utility function, but we want to ensure it handles edge cases
// correctly since it's used in RenderLog for drawing dividers.
func TestStringsRepeat(t *testing.T) {
	if got := stringsRepeat("x", 0); got != "" {
		t.Fatalf("expected empty string for n=0, got %q", got)
	}

	if got := stringsRepeat("ab", 3); got != "ababab" {
		t.Fatalf("unexpected repeat result: %q", got)
	}

	if got := stringsRepeat("z", -5); got != "" {
		t.Fatalf("expected empty string for negative n, got %q", got)
	}
}

// TestRenderLog_EmptyAndOffsetFixup ensures RenderLog behaves correctly with
// no log entries and that it adjusts the provided offset to a valid value.
func TestRenderLog_EmptyAndOffsetFixup(t *testing.T) {
	var offset = -10
	agg := mockAgg{logs: nil}

	out := RenderLog(50, 6, agg, &offset)

	if !strings.Contains(out, "live log") {
		t.Fatalf("output missing title; got: %q", out)
	}

	if !strings.Contains(out, "(0 entries)") {
		t.Fatalf("expected entries count in title; got: %q", out)
	}

	// bodyH := max(h-3, 1) with h=6 => 3; since n=0 offset must be 0
	if offset != 0 {
		t.Fatalf("expected offset adjusted to 0 for empty entries, got %d", offset)
	}
}

// TestRenderLog_WithEntries_TruncationAndErrorTag verifies RenderLog output
// when there are entries: timestamps, error tags and truncation/ellipsis.
func TestRenderLog_WithEntries_TruncationAndErrorTag(t *testing.T) {
	// two entries: one normal, one with error and long args
	t1 := time.Date(2023, 1, 2, 12, 34, 56, 0, time.UTC)
	t2 := t1.Add(time.Second)

	longArgs := strings.Repeat("verylongarg/", 10)

	entries := []aggregator.LogEntry{
		{Time: t1, PID: 1, Name: "open", Args: "/some/path", RetVal: "0", Error: ""},
		{Time: t2, PID: 2, Name: "read", Args: longArgs, RetVal: "-1", Error: "enoent"},
	}

	agg := mockAgg{logs: entries}
	var offset = 0

	// small width/height to force truncation
	out := RenderLog(40, 6, agg, &offset)

	// title should mention 2 entries
	if !strings.Contains(out, "(2 entries)") {
		t.Fatalf("expected 2 entries in title; got: %q", out)
	}

	// timestamps should be present (formatted as HH:MM:SS)
	if !strings.Contains(out, "12:34:56") || !strings.Contains(out, "12:34:57") {
		t.Fatalf("expected timestamps in output; got: %q", out)
	}

	// error tag (ERR) should appear for the entry with Error
	if !strings.Contains(out, "ERR") {
		t.Fatalf("expected error tag in output for error entry; got: %q", out)
	}

	// long args should be truncated and show an ellipsis
	if !strings.Contains(out, "…") {
		t.Fatalf("expected truncated args with ellipsis; got: %q", out)
	}
}
