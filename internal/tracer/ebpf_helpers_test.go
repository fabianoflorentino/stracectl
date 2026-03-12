//go:build ebpf
// +build ebpf

package tracer

import "testing"

func TestErrnoName(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{2, "ENOENT"},
		{13, "EACCES"},
		{11, "EAGAIN"},
		{9999, ""},
	}

	for _, tt := range tests {
		if got := ErrnoName(tt.n); got != tt.want {
			t.Fatalf("ErrnoName(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestFormatArg(t *testing.T) {
	if formatArg(0) != "0" {
		t.Fatalf("formatArg(0) != 0")
	}
	if formatArg(10) != "10" {
		t.Fatalf("formatArg(10) != 10")
	}
	if formatArg(0x7fffe000) != "0x7fffe000" {
		t.Fatalf("formatArg(ptr) hex expected")
	}
}
