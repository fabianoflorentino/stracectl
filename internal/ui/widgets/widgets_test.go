package widgets

import (
	"testing"
	"unicode/utf8"
)

func Test_WordWrap_TruncateAndSanitize_SparkBar_Cols(t *testing.T) {
	text := "one two three four five six"
	lines := WordWrap(text, 10)
	if len(lines) < 2 {
		t.Fatalf("expected multiple lines, got %v", lines)
	}
	for _, l := range lines {
		if utf8.RuneCountInString(l) > 10 {
			t.Fatalf("line too long: %q", l)
		}
	}

	s := "hello world"
	out := TruncateToWidth(s, 5)
	if len(out) == 0 {
		t.Fatalf("truncate returned empty")
	}

	// Sanitize preserves tabs and replaces control chars
	in := "a\t\x01b"
	got := SanitizeForTUI(in)
	if !containsTab(got) {
		t.Fatalf("tab should be preserved: %q", got)
	}

	sb := SparkBar(3, 10, 8)
	if utf8.RuneCountInString(sb) != 8 {
		t.Fatalf("expected width 8, got %d", utf8.RuneCountInString(sb))
	}

	cw := ColWidths(60)
	if cw.Name <= 0 || cw.File <= 0 {
		t.Fatalf("invalid col widths: %+v", cw)
	}
}

func containsTab(s string) bool {
	for _, r := range s {
		if r == '\t' {
			return true
		}
	}
	return false
}
