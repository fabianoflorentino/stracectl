package terminal

import (
	"os"
	"testing"
)

func Test_SafeIntAndDetectFallback(t *testing.T) {
	if v, ok := SafeIntFromUintptr(1); !ok || v != 1 {
		t.Fatalf("SafeIntFromUintptr small value failed")
	}

	// set env vars to force env-based fallback
	os.Setenv("COLUMNS", "120")
	os.Setenv("LINES", "40")
	w, h := DetectFallbackSize()
	if w <= 0 || h <= 0 {
		t.Fatalf("DetectFallbackSize returned invalid values: %d x %d", w, h)
	}
}
