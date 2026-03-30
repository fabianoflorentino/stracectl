package aggregator

import (
	"encoding/json"
	"testing"
	"time"
)

// TestCategory_Classification verifies that known syscalls are classified into
// the correct categories and that unknown syscalls are classified as OTHER.
func TestCategory_Classification(t *testing.T) {
	cases := []struct {
		syscall string
		wantStr string
	}{
		{"read", "I/O"},
		{"openat", "I/O"},
		{"fstat", "FS"},
		{"connect", "NET"},
		{"mmap", "MEM"},
		{"execve", "PROC"},
		{"rt_sigaction", "SIG"},
		{"unknownsyscall", "OTHER"},
	}

	for _, tc := range cases {
		a := New()
		a.Add(ok(tc.syscall, 1*time.Microsecond))

		stats := a.Sorted(SortByCount)
		if len(stats) == 0 {
			t.Fatalf("%s: no stats returned", tc.syscall)
		}

		if stats[0].Category.String() != tc.wantStr {
			t.Errorf("%s: want %s, got %s", tc.syscall, tc.wantStr, stats[0].Category.String())
		}
	}
}

// TestCategoryJSON verifies that Category values can be correctly marshaled to and unmarshaled from JSON.
func TestCategoryJSON(t *testing.T) {
	b, err := json.Marshal(CatIO)

	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var c Category

	if err := json.Unmarshal(b, &c); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if c != CatIO {
		t.Fatalf("roundtrip failed: got %v", c)
	}
}
