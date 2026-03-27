package widgets

import (
	"testing"
	"unicode/utf8"
)

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
