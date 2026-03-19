package ui

import (
	"os"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestWordWrap(t *testing.T) {
	text := "one two three four five six"
	lines := wordWrap(text, 10)
	if len(lines) < 2 {
		t.Fatalf("expected multiple lines, got %v", lines)
	}
	for _, l := range lines {
		if utf8.RuneCountInString(l) > 10 {
			t.Fatalf("line too long: %q", l)
		}
	}
}

func TestTruncateAndSanitize(t *testing.T) {
	s := "hello world"
	out := truncateToWidth(s, 5)
	if !strings.Contains(out, "…") {
		t.Fatalf("expected ellipsis, got %q", out)
	}

	// control chars replaced, tabs kept
	in := "a\t\x01b"
	got := sanitizeForTUI(in)
	if !strings.Contains(got, "\t") {
		t.Fatalf("tab should be preserved: %q", got)
	}
	if strings.Contains(got, "\x01") {
		t.Fatalf("control char should be removed: %q", got)
	}
}

func TestFormatDurAndCount(t *testing.T) {
	if formatDur(0) != "—" {
		t.Fatalf("expected em dash for zero")
	}
	if !strings.HasSuffix(formatDur(500), "ns") {
		t.Fatalf("expected ns suffix: %q", formatDur(500))
	}
	if !strings.Contains(formatDur(1500), "µs") {
		t.Fatalf("expected µs formatting: %q", formatDur(1500))
	}
	if !strings.Contains(formatDur(5*1e6), "ms") {
		t.Fatalf("expected ms formatting: %q", formatDur(5*1e6))
	}

	if formatCount(1) != "1" {
		t.Fatalf("unexpected formatCount(1): %s", formatCount(1))
	}
	if !strings.HasSuffix(formatCount(1500), "k") {
		t.Fatalf("expected k suffix: %q", formatCount(1500))
	}
	if !strings.HasSuffix(formatCount(2_000_000), "M") {
		t.Fatalf("expected M suffix: %q", formatCount(2_000_000))
	}
}

func TestSparkBarAndCols(t *testing.T) {
	sb := sparkBar(3, 10, 8)
	if utf8.RuneCountInString(sb) != 8 {
		t.Fatalf("expected width 8, got %d", utf8.RuneCountInString(sb))
	}
	if !strings.Contains(sb, "█") {
		t.Fatalf("expected filled char in sparkBar: %q", sb)
	}

	cw := colWidths(60)
	// basic sanity checks
	if cw.name <= 0 || cw.file <= 0 {
		t.Fatalf("invalid col widths: %+v", cw)
	}
}

func TestSafeIntAndDetectFallback(t *testing.T) {
	if v, ok := safeIntFromUintptr(1); !ok || v != 1 {
		t.Fatalf("safeIntFromUintptr small value failed")
	}

	// set env vars to force the env-based fallback
	os.Setenv("COLUMNS", "120")
	os.Setenv("LINES", "40")
	w, h := detectFallbackSize()
	if w <= 0 || h <= 0 {
		t.Fatalf("detectFallbackSize returned invalid values: %d x %d", w, h)
	}
}
