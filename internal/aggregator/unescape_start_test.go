package aggregator

import (
	"testing"
	"time"
)

func TestStartTime_NonZero(t *testing.T) {
	a := New()
	st := a.StartTime()
	if st.IsZero() {
		t.Fatalf("StartTime returned zero time")
	}
	// sanity: should be recent
	if time.Since(st) > time.Minute {
		t.Fatalf("StartTime too old: %v", st)
	}
}

func TestUnescapePath_Behaviour(t *testing.T) {
	// Unquote("\\x00") -> NUL -> should be rejected
	if got := unescapePath("\\x00"); got != "" {
		t.Fatalf("expected empty for NUL escape, got %q", got)
	}

	// Unquote fails for trailing backslash; fallback should return raw input
	if got := unescapePath("bad\\"); got != "bad\\" {
		t.Fatalf("expected fallback raw string, got %q", got)
	}

	// Normal path returned unchanged
	if got := unescapePath("/tmp/foo"); got != "/tmp/foo" {
		t.Fatalf("expected /tmp/foo, got %q", got)
	}

	// Hex escape should unquote to the correct character
	if got := unescapePath("\\x41"); got != "A" {
		t.Fatalf("expected A, got %q", got)
	}
}
