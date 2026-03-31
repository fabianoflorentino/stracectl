package render

import (
	"strings"
	"testing"
)

// TestAlertExplanation verifies that AlertExplanation returns non-empty explanations
// for known syscall names and that the explanations contain expected substrings.
func TestAlertExplanation(t *testing.T) {
	cases := map[string]string{
		"open":              "files not found",
		"openat":            "files not found",
		"open_by_handle_at": "files not found",
		"recvmsg":           "EAGAIN on non-blocking socket",
		"recvfrom":          "EAGAIN on non-blocking socket",
		"send":              "send failed",
		"sendmsg":           "send failed",
		"mkdirat":           "directory already exists",
		"fstatfs":           "filesystem stat failed",
		"ioctl":             "terminal control failed",
		"access":            "optional files are missing",
		"faccessat":         "optional files are missing",
	}

	for name, wantSub := range cases {
		t.Run(name, func(t *testing.T) {
			got := AlertExplanation(name)

			if got == "" {
				t.Fatalf("%s: got empty explanation", name)
			}

			if !strings.Contains(got, wantSub) {
				t.Fatalf("%s: got=%q, want contains %q", name, got, wantSub)
			}
		})
	}
}
