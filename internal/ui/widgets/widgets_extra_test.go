package widgets

import "testing"

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
