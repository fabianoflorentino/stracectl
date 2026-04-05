package overlays

import (
	"strings"
	"testing"

	"github.com/fabianoflorentino/stracectl/internal/aggregator"
	"github.com/fabianoflorentino/stracectl/internal/procinfo"
)

// ── helper stubs ─────────────────────────────────────────────────────────────

type emptyFilesAgg struct{}

func (e *emptyFilesAgg) Total() int64                                           { return 0 }
func (e *emptyFilesAgg) Errors() int64                                          { return 0 }
func (e *emptyFilesAgg) Rate() float64                                          { return 0 }
func (e *emptyFilesAgg) UniqueCount() int                                       { return 0 }
func (e *emptyFilesAgg) Sorted(_ aggregator.SortField) []aggregator.SyscallStat { return nil }
func (e *emptyFilesAgg) RecentLog() []aggregator.LogEntry                       { return nil }
func (e *emptyFilesAgg) CategoryBreakdown() map[aggregator.Category]aggregator.CategoryStats {
	return nil
}
func (e *emptyFilesAgg) GetProcInfo() procinfo.ProcInfo       { return procinfo.ProcInfo{} }
func (e *emptyFilesAgg) TopFiles(_ int) []aggregator.FileStat { return nil }

type singleFileAgg struct {
	path  string
	count int64
}

func (s *singleFileAgg) Total() int64                                           { return 0 }
func (s *singleFileAgg) Errors() int64                                          { return 0 }
func (s *singleFileAgg) Rate() float64                                          { return 0 }
func (s *singleFileAgg) UniqueCount() int                                       { return 0 }
func (s *singleFileAgg) Sorted(_ aggregator.SortField) []aggregator.SyscallStat { return nil }
func (s *singleFileAgg) RecentLog() []aggregator.LogEntry                       { return nil }
func (s *singleFileAgg) CategoryBreakdown() map[aggregator.Category]aggregator.CategoryStats {
	return nil
}
func (s *singleFileAgg) GetProcInfo() procinfo.ProcInfo { return procinfo.ProcInfo{} }
func (s *singleFileAgg) TopFiles(_ int) []aggregator.FileStat {
	return []aggregator.FileStat{{Path: s.path, Count: s.count}}
}

// ── helpersFormatCount ────────────────────────────────────────────────────────

// TestHelpersFormatCount_Units verifies that helpersFormatCount correctly formats
// counts into human-readable strings with appropriate units (k, M, etc.).
func TestHelpersFormatCount_Units(t *testing.T) {
	tests := []struct {
		n    int64
		want string
	}{
		{0, "0"},
		{999, "999"},
		{1_000, "1.0k"},
		{1_500, "1.5k"},
		{999_999, "1000.0k"},
		{1_000_000, "1.0M"},
		{2_500_000, "2.5M"},
	}

	for _, tc := range tests {
		got := helpersFormatCount(tc.n)
		if got != tc.want {
			t.Errorf("helpersFormatCount(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

// ── RenderFiles ───────────────────────────────────────────────────────────────

// TestRenderFiles_Basic verifies that RenderFiles returns a non-empty string without panicking.
func TestRenderFiles_ReturnsNonEmpty(t *testing.T) {
	off := 0
	out := RenderFiles(80, 10, &fakeAgg{}, &off)
	if out == "" {
		t.Fatal("expected non-empty output")
	}
}

// TestRenderFiles_DefaultDimensions verifies that RenderFiles uses fallback dimensions when w or h is zero.
func TestRenderFiles_DefaultDimensions(t *testing.T) {
	// w=0, h=0 should use fallback 80×24 without panicking
	off := 0
	out := RenderFiles(0, 0, &fakeAgg{}, &off)
	if out == "" {
		t.Fatal("expected non-empty output with zero dimensions")
	}
}

// TestRenderFiles_ContainsTitle verifies that the output contains the expected title string.
func TestRenderFiles_ContainsTitle(t *testing.T) {
	off := 0
	out := RenderFiles(80, 10, &fakeAgg{}, &off)
	if !strings.Contains(out, "top files") {
		t.Errorf("output does not contain 'top files': %q", out)
	}
}

// TestRenderFiles_ContainsEntryCount verifies that the output includes the correct entry count from the aggregator.
func TestRenderFiles_ContainsEntryCount(t *testing.T) {
	agg := &fakeAgg{} // returns 2 entries
	off := 0
	out := RenderFiles(80, 10, agg, &off)
	if !strings.Contains(out, "2 entries") {
		t.Errorf("output does not contain entry count: %q", out)
	}
}

// TestRenderFiles_ContainsFilePath verifies that the output includes file paths from the aggregator's TopFiles.
func TestRenderFiles_ContainsFilePath(t *testing.T) {
	off := 0
	out := RenderFiles(80, 10, &fakeAgg{}, &off)
	if !strings.Contains(out, "/etc/passwd") {
		t.Errorf("output does not contain '/etc/passwd': %q", out)
	}
}

// TestRenderFiles_ContainsFooter verifies that the output includes the expected footer hint for quitting.
func TestRenderFiles_ContainsFooter(t *testing.T) {
	off := 0
	out := RenderFiles(80, 10, &fakeAgg{}, &off)
	if !strings.Contains(out, "q:quit") {
		t.Errorf("output does not contain footer hint 'q:quit': %q", out)
	}
}

// TestRenderFiles_FormattedCountVisible verifies that the formatted count (e.g., "1.2k")
// is visible in the output when the aggregator returns a count that should be formatted.
func TestRenderFiles_FormattedCountVisible(t *testing.T) {
	off := 0
	out := RenderFiles(80, 10, &fakeAgg{}, &off)
	// fakeAgg returns Count:1234 → "1.2k"
	if !strings.Contains(out, "1.2k") {
		t.Errorf("expected formatted count '1.2k' in output: %q", out)
	}
}

// TestRenderFiles_OffsetClamping verifies that the filesOffset is clamped to valid ranges and does not cause panics.
func TestRenderFiles_EmptyList(t *testing.T) {
	stub := &emptyFilesAgg{}
	off := 0
	out := RenderFiles(80, 10, stub, &off)
	if out == "" {
		t.Fatal("expected non-empty output even with no files")
	}
	if !strings.Contains(out, "0 entries") {
		t.Errorf("expected '0 entries' in output: %q", out)
	}
}

// TestRenderFiles_OffsetClampedToZero verifies that a negative filesOffset is
// clamped to zero and does not cause panics.
func TestRenderFiles_OffsetClampedToZero(t *testing.T) {
	off := -5
	// Should not panic and offset should be clamped
	out := RenderFiles(80, 10, &fakeAgg{}, &off)
	if out == "" {
		t.Fatal("expected non-empty output")
	}
	if off != 0 {
		t.Errorf("expected offset to be clamped to 0, got %d", off)
	}
}

// TestRenderFiles_OffsetBeyondEnd verifies that a filesOffset greater than n-bodyH
// is clamped to zero and does not cause panics.
func TestRenderFiles_OffsetBeyondEnd_ClampedToZero(t *testing.T) {
	off := 9999
	out := RenderFiles(80, 10, &fakeAgg{}, &off)
	if out == "" {
		t.Fatal("expected non-empty output")
	}
	if off != 0 {
		t.Errorf("expected offset clamped to 0 when > n-bodyH, got %d", off)
	}
}

// TestRenderFiles_LongPathTruncated verifies that when a file path exceeds the available
// width, it is truncated in the output and does not appear verbatim.
func TestRenderFiles_LongPathTruncated(t *testing.T) {
	longPath := strings.Repeat("a", 200)
	stub := &singleFileAgg{path: longPath, count: 1}
	off := 0
	out := RenderFiles(80, 10, stub, &off)
	// The full path must not appear verbatim since it exceeds available width
	if strings.Contains(out, longPath) {
		t.Error("long path was not truncated in output")
	}
}
