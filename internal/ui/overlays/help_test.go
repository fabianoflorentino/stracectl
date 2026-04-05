package overlays

import (
	"strings"
	"testing"
)

// TestRenderHelp_ReturnsNonEmpty verifies that RenderHelp returns a non-empty string.
// This ensures that the function is producing output and not crashing or returning an empty overlay.
func TestRenderHelp_ReturnsNonEmpty(t *testing.T) {
	out := RenderHelp(80)
	if out == "" {
		t.Fatal("expected non-empty output")
	}
}

// TestRenderHelp_ZeroWidth_NoPanic verifies that RenderHelp does not panic when called with w=0.
// The function must handle degenerate dimensions gracefully and still return usable output.
func TestRenderHelp_ZeroWidth_NoPanic(t *testing.T) {
	// should not panic with zero or very small width
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panic with w=0: %v", r)
		}
	}()
	out := RenderHelp(0)
	if out == "" {
		t.Fatal("expected non-empty output with w=0")
	}
}

// TestRenderHelp_ContainsTitle verifies that the rendered output contains the application name
// and the dismiss hint so the user knows how to close the overlay.
func TestRenderHelp_ContainsTitle(t *testing.T) {
	out := RenderHelp(80)
	if !strings.Contains(out, "stracectl") {
		t.Error("output does not contain 'stracectl' in title")
	}
	if !strings.Contains(out, "press any key to close") {
		t.Error("output does not contain close hint in title")
	}
}

// TestRenderHelp_ContainsSectionHeaders verifies that all five expected section headings
// (COLUMNS, ROW COLOURS, CATEGORY BAR, COMMON PATTERNS, KEYBOARD SHORTCUTS) appear in the output.
func TestRenderHelp_ContainsSectionHeaders(t *testing.T) {
	out := RenderHelp(80)
	sections := []string{
		"COLUMNS",
		"ROW COLOURS",
		"CATEGORY BAR",
		"COMMON PATTERNS",
		"KEYBOARD SHORTCUTS",
	}
	for _, s := range sections {
		if !strings.Contains(out, s) {
			t.Errorf("output does not contain section %q", s)
		}
	}
}

// TestRenderHelp_ContainsColumnNames verifies that every column name documented in the COLUMNS
// section is present, ensuring none were accidentally dropped from the help text.
func TestRenderHelp_ContainsColumnNames(t *testing.T) {
	out := RenderHelp(80)
	columns := []string{"SYSCALL", "FILE", "CAT", "CALLS", "FREQ", "AVG", "MAX", "TOTAL", "ERRORS", "ERR%"}
	for _, col := range columns {
		if !strings.Contains(out, col) {
			t.Errorf("output does not contain column %q", col)
		}
	}
}

// TestRenderHelp_ContainsRowColourDescriptions verifies that each row colour label
// (white, yellow, orange, red) appears in the ROW COLOURS section of the output.
func TestRenderHelp_ContainsRowColourDescriptions(t *testing.T) {
	out := RenderHelp(80)
	colours := []string{"white", "yellow", "orange", "red"}
	for _, c := range colours {
		if !strings.Contains(out, c) {
			t.Errorf("output does not contain row colour %q", c)
		}
	}
}

// TestRenderHelp_ContainsCategoryBarEntries verifies that every syscall category
// (I/O, FS, NET, MEM, PROC, SIG) is described in the CATEGORY BAR section.
func TestRenderHelp_ContainsCategoryBarEntries(t *testing.T) {
	out := RenderHelp(80)
	cats := []string{"I/O", "FS", "NET", "MEM", "PROC", "SIG"}
	for _, cat := range cats {
		if !strings.Contains(out, cat) {
			t.Errorf("output does not contain category %q", cat)
		}
	}
}

// TestRenderHelp_ContainsCommonPatterns verifies that all eight common syscall patterns
// are listed under COMMON PATTERNS, helping users interpret noisy but benign error rates.
func TestRenderHelp_ContainsCommonPatterns(t *testing.T) {
	out := RenderHelp(80)
	patterns := []string{"openat ERR%", "recvfrom ERR%", "connect ERR%", "ioctl ERR%", "madvise ERR%", "high I/O%", "high FS%", "high SIG%"}
	for _, p := range patterns {
		if !strings.Contains(out, p) {
			t.Errorf("output does not contain pattern %q", p)
		}
	}
}

// TestRenderHelp_ContainsKeyboardShortcuts verifies that the primary navigation bindings
// (arrow keys, enter/d, and quit) are documented in the KEYBOARD SHORTCUTS section.
func TestRenderHelp_ContainsKeyboardShortcuts(t *testing.T) {
	out := RenderHelp(80)
	shortcuts := []string{"↑ / k", "↓ / j", "enter / d", "q / Ctrl+C"}
	for _, kk := range shortcuts {
		if !strings.Contains(out, kk) {
			t.Errorf("output does not contain shortcut %q", kk)
		}
	}
}

// TestRenderHelp_ContainsSortKeys verifies that every single-key sort binding
// (c, t, a, x, e, n, g, /, esc, ?) appears in the KEYBOARD SHORTCUTS section.
func TestRenderHelp_ContainsSortKeys(t *testing.T) {
	out := RenderHelp(80)
	keys := []string{" c ", " t ", " a ", " x ", " e ", " n ", " g ", " / ", "esc", " ? "}
	for _, k := range keys {
		if !strings.Contains(out, k) {
			t.Errorf("output does not contain sort key %q", k)
		}
	}
}

// TestRenderHelp_ContainsFooter verifies that the footer dismiss prompt is present at the
// bottom of the overlay so the user always has a visible cue to close the help screen.
func TestRenderHelp_ContainsFooter(t *testing.T) {
	out := RenderHelp(80)
	if !strings.Contains(out, "press any key to return") {
		t.Error("output does not contain footer text")
	}
}

// TestRenderHelp_WidthConsistency verifies that RenderHelp is deterministic: calling it
// twice with the same width must always produce identical output.
func TestRenderHelp_WidthConsistency(t *testing.T) {
	// same width must always produce the same output
	out1 := RenderHelp(100)
	out2 := RenderHelp(100)
	if out1 != out2 {
		t.Error("RenderHelp is not deterministic for the same width")
	}
}

// TestRenderHelp_DifferentWidths verifies that the rendered output changes when the terminal
// width changes, confirming that the title and divider adapt to the available space.
func TestRenderHelp_DifferentWidths(t *testing.T) {
	// output for different widths should differ (title/divider width changes)
	out40 := RenderHelp(40)
	out120 := RenderHelp(120)
	if out40 == out120 {
		t.Error("expected output to differ for different widths")
	}
}
