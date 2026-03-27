package helpers

import (
	"strings"
	"testing"
	"time"
)

func Test_FormatDur_And_FormatCount(t *testing.T) {
	if FormatDur(0) != "—" {
		t.Fatalf("expected em dash for zero")
	}
	if !strings.HasSuffix(FormatDur(500), "ns") {
		t.Fatalf("expected ns suffix: %q", FormatDur(500))
	}
	if !strings.Contains(FormatDur(1500), "µs") {
		t.Fatalf("expected µs formatting: %q", FormatDur(1500))
	}
	if !strings.Contains(FormatDur(5*1e6), "ms") {
		t.Fatalf("expected ms formatting: %q", FormatDur(5*1e6))
	}

	if FormatCount(1) != "1" {
		t.Fatalf("unexpected FormatCount(1): %s", FormatCount(1))
	}
	if !strings.HasSuffix(FormatCount(1500), "k") {
		t.Fatalf("expected k suffix: %q", FormatCount(1500))
	}
	if !strings.HasSuffix(FormatCount(2_000_000), "M") {
		t.Fatalf("expected M suffix: %q", FormatCount(2_000_000))
	}

	// quick sanity for durations
	_ = FormatDur(time.Millisecond * 123)
}
