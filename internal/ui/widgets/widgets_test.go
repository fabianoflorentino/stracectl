package widgets

import (
	"strings"
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

func TestSanitizeForTUI_Empty(t *testing.T) {
	if SanitizeForTUI("") != "" {
		t.Fatalf("expected empty input to return empty")
	}
}

func TestSanitizeForTUI_ControlCharsReplaced(t *testing.T) {
	// include NUL, BEL, DEL and other control chars
	in := "hello\x00world\x07!\x7f"
	out := SanitizeForTUI(in)
	if out == in {
		t.Fatalf("expected control characters to be replaced, got identical output")
	}
	if len(out) == 0 {
		t.Fatalf("unexpected empty output")
	}
	// ensure replacement rune present (U+FFFD)
	if !containsRune(out, '\uFFFD') {
		t.Fatalf("expected replacement rune in output: %q", out)
	}
}

func TestSanitizeForTUI_TabsPreserved(t *testing.T) {
	in := "col1\tcol2\tcol3"
	out := SanitizeForTUI(in)
	if out != in {
		t.Fatalf("expected tabs preserved, got %q", out)
	}
}

func TestSanitizeForTUI_MultibyteKept(t *testing.T) {
	in := "pässwörd — пример \x00"
	out := SanitizeForTUI(in)
	// multibyte characters should remain present
	if !containsRune(out, 'п') || !containsRune(out, 'ä') {
		t.Fatalf("expected multibyte runes preserved in output: %q", out)
	}
	// control char replaced
	if !containsRune(out, '\uFFFD') {
		t.Fatalf("expected replacement rune in output: %q", out)
	}
}

func containsRune(s string, r rune) bool {
	for _, rr := range s {
		if rr == r {
			return true
		}
	}
	return false
}

func Test_ColWidths_WideAndMedium(t *testing.T) {
	c := ColWidths(200)
	if c.Name < 18 || c.File < 30 {
		t.Fatalf("unexpected widths for wide terminal: %+v", c)
	}
	c2 := ColWidths(60)
	if c2.Name < 14 || c2.File < 8 {
		t.Fatalf("unexpected widths for medium/narrow terminal: %+v", c2)
	}
}

func Test_TruncateToWidth_Runes(t *testing.T) {
	s := "こんにちは世界" // multibyte runes
	out := TruncateToWidth(s, 5)
	if out == s && utf8.RuneCountInString(s) > 2 {
		t.Fatalf("expected truncation for multibyte string: %q -> %q", s, out)
	}
}

func Test_SparkBar_EdgeCases(t *testing.T) {
	if SparkBar(0, 0, 5) != "░░░░░" {
		t.Fatalf("expected empty bar when maxCount=0")
	}
	bar := SparkBar(100, 50, 4)
	if len([]rune(bar)) != 4 {
		t.Fatalf("unexpected sparkbar width: %q", bar)
	}
}

func Test_SanitizeForTUI_PreservesTab(t *testing.T) {
	in := "a\tb\x00c"
	out := SanitizeForTUI(in)
	if out == in {
		t.Fatalf("expected replacement of NUL in %q", in)
	}
	if out[1] != '\t' {
		t.Fatalf("expected tab preserved: got %q", out)
	}
}

func Test_ColWidths_Narrow(t *testing.T) {
	c := ColWidths(50)
	if c.Name == 0 || c.File == 0 {
		t.Fatalf("unexpected zero column widths: %+v", c)
	}
}

func Test_TruncateAndPad(t *testing.T) {
	s := "hello world"
	if TruncateToWidth(s, 5) == s {
		t.Fatalf("expected truncation for width 5")
	}
	if PadR("hi", 5) != "hi   " {
		t.Fatalf("PadR failed")
	}
	if PadL("hi", 5) != "   hi" {
		t.Fatalf("PadL failed")
	}
}

func Test_SparkBarAndWordWrap(t *testing.T) {
	bar := SparkBar(5, 10, 4)
	if len([]rune(bar)) != 4 {
		t.Fatalf("SparkBar width mismatch: %q", bar)
	}
	lines := WordWrap("one two three four", 7)
	if len(lines) < 2 {
		t.Fatalf("WordWrap expected multiple lines, got %v", lines)
	}
}

func Test_SanitizeForTUI(t *testing.T) {
	in := "hello\x00world\t!"
	out := SanitizeForTUI(in)
	if out == in {
		t.Fatalf("expected control chars replaced in %q", in)
	}
}

func TestColWidths_FallbackNarrow(t *testing.T) {
	c := ColWidths(50)
	if c.Name != 14 || c.File != 8 {
		t.Fatalf("expected Name=14 File=8 for narrow width 50, got %+v", c)
	}
}

func TestColWidths_AdjustMedium(t *testing.T) {
	c := ColWidths(110)
	if c.Name != 18 || c.File != 7 {
		t.Fatalf("expected Name=18 File=7 for medium width 110, got %+v", c)
	}
}

func TestColWidths_Wide(t *testing.T) {
	c := ColWidths(200)
	if c.Name < 18 || c.File < 30 {
		t.Fatalf("expected wide layout with Name>=18 File>=30, got %+v", c)
	}
}

func TestTruncateToWidth_EmptyWidth(t *testing.T) {
	if TruncateToWidth("abc", 0) != "" {
		t.Fatalf("expected empty string when width <= 0")
	}
}

func TestTruncateToWidth_NoTruncate(t *testing.T) {
	s := "abc"
	if out := TruncateToWidth(s, 5); out != s {
		t.Fatalf("expected no truncation, got %q", out)
	}
}

func TestTruncateToWidth_MultibyteTruncate(t *testing.T) {
	s := "こんにちは世界"
	out := TruncateToWidth(s, 5)
	if !strings.HasSuffix(out, "…") {
		t.Fatalf("expected ellipsis suffix when truncated, got %q", out)
	}
	if !utf8.ValidString(out) {
		t.Fatalf("output contains invalid UTF-8: %q", out)
	}
}

func TestPadR_NoPad(t *testing.T) {
	if out := PadR("hello", 3); out != "hello" {
		t.Fatalf("expected no padding when target < width, got %q", out)
	}
}

func TestPadR_Pad(t *testing.T) {
	if PadR("hi", 5) != "hi   " {
		t.Fatalf("PadR failed to pad right")
	}
}

func TestPadL_NoPad(t *testing.T) {
	if out := PadL("hello", 3); out != "hello" {
		t.Fatalf("expected no padding when target < width, got %q", out)
	}
}

func TestPadL_Pad(t *testing.T) {
	if PadL("hi", 5) != "   hi" {
		t.Fatalf("PadL failed to pad left")
	}
}

func TestWordWrap_ZeroWidth(t *testing.T) {
	in := "short text"
	out := WordWrap(in, 0)
	if len(out) != 1 || out[0] != in {
		t.Fatalf("expected original text returned for zero width, got %v", out)
	}
}

func TestWordWrap_ShortText(t *testing.T) {
	in := "one two"
	out := WordWrap(in, 20)
	if len(out) != 1 || out[0] != in {
		t.Fatalf("expected single-line output for short text, got %v", out)
	}
}
