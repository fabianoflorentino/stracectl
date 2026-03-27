package ui

import (
	"os"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/fabianoflorentino/stracectl/internal/ui/helpers"
	"github.com/fabianoflorentino/stracectl/internal/ui/terminal"
	"github.com/fabianoflorentino/stracectl/internal/ui/widgets"
)

func TestWordWrap(t *testing.T) {
	text := "one two three four five six"
	lines := widgets.WordWrap(text, 10)
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
	out := widgets.TruncateToWidth(s, 5)
	if !strings.Contains(out, "…") {
		t.Fatalf("expected ellipsis, got %q", out)
	}

	// control chars replaced, tabs kept
	in := "a\t\x01b"
	got := widgets.SanitizeForTUI(in)
	if !strings.Contains(got, "\t") {
		t.Fatalf("tab should be preserved: %q", got)
	}
	if strings.Contains(got, "\x01") {
		t.Fatalf("control char should be removed: %q", got)
	}
}

func TestFormatDurAndCount(t *testing.T) {
	if helpers.FormatDur(0) != "—" {
		t.Fatalf("expected em dash for zero")
	}
	if !strings.HasSuffix(helpers.FormatDur(500), "ns") {
		t.Fatalf("expected ns suffix: %q", helpers.FormatDur(500))
	}
	if !strings.Contains(helpers.FormatDur(1500), "µs") {
		t.Fatalf("expected µs formatting: %q", helpers.FormatDur(1500))
	}
	if !strings.Contains(helpers.FormatDur(5*1e6), "ms") {
		t.Fatalf("expected ms formatting: %q", helpers.FormatDur(5*1e6))
	}

	if helpers.FormatCount(1) != "1" {
		t.Fatalf("unexpected FormatCount(1): %s", helpers.FormatCount(1))
	}
	if !strings.HasSuffix(helpers.FormatCount(1500), "k") {
		t.Fatalf("expected k suffix: %q", helpers.FormatCount(1500))
	}
	if !strings.HasSuffix(helpers.FormatCount(2_000_000), "M") {
		t.Fatalf("expected M suffix: %q", helpers.FormatCount(2_000_000))
	}
}

func TestSparkBarAndCols(t *testing.T) {
	sb := widgets.SparkBar(3, 10, 8)
	if utf8.RuneCountInString(sb) != 8 {
		t.Fatalf("expected width 8, got %d", utf8.RuneCountInString(sb))
	}
	if !strings.Contains(sb, "█") {
		t.Fatalf("expected filled char in sparkBar: %q", sb)
	}

	cw := widgets.ColWidths(60)
	// basic sanity checks
	if cw.Name <= 0 || cw.File <= 0 {
		t.Fatalf("invalid col widths: %+v", cw)
	}
}

func TestSafeIntAndDetectFallback(t *testing.T) {
	if v, ok := terminal.SafeIntFromUintptr(1); !ok || v != 1 {
		t.Fatalf("safeIntFromUintptr small value failed")
	}

	// set env vars to force the env-based fallback
	os.Setenv("COLUMNS", "120")
	os.Setenv("LINES", "40")
	w, h := terminal.DetectFallbackSize()
	if w <= 0 || h <= 0 {
		t.Fatalf("detectFallbackSize returned invalid values: %d x %d", w, h)
	}
}
