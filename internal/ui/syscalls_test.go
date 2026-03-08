package ui

import (
	"fmt"
	"testing"
)

// TestSyscallAliasesPointToValidCanonicals verifies that every entry in
// syscallAliases resolves to a key that actually exists in syscallDetails.
func TestSyscallAliasesPointToValidCanonicals(t *testing.T) {
	for alias, canonical := range syscallAliases {
		if _, ok := syscallDetails[canonical]; !ok {
			t.Errorf("syscallAliases[%q] = %q, but %q has no entry in syscallDetails", alias, canonical, canonical)
		}
	}
}

// TestSyscallInfoKnownNames verifies that syscallInfo returns the expected
// description for a canonical name and for its aliases.
func TestSyscallInfoKnownNames(t *testing.T) {
	cases := []struct {
		input    string
		wantDesc string
	}{
		{"read", "Read bytes from a file descriptor into a buffer."},
		{"write", "Write bytes from a buffer to a file descriptor."},
		// aliases
		{"open", "Open or create a file, returning a file descriptor."},              // → openat
		{"mmap2", "Map files or devices into memory, or allocate anonymous memory."}, // → mmap
		{"stat", "Retrieve file metadata (size, permissions, timestamps, inode)."},   // → fstat
		{"recv", "Receive data from a socket."},                                      // → recvfrom
		{"send", "Send data through a socket."},                                      // → sendto
		{"sigaction", "Install or query a signal handler."},                          // → rt_sigaction
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := syscallInfo(tc.input)
			if got.description != tc.wantDesc {
				t.Errorf("syscallInfo(%q).description =\n  %q\nwant:\n  %q", tc.input, got.description, tc.wantDesc)
			}
		})
	}
}

// TestSyscallInfoUnknownName verifies that an unrecognised syscall returns a
// non-empty generic description instead of a zero value.
func TestSyscallInfoUnknownName(t *testing.T) {
	name := "totally_unknown_syscall_xyz"
	got := syscallInfo(name)
	if got.description == "" {
		t.Errorf("syscallInfo(%q).description is empty; expected a generic fallback", name)
	}
	want := fmt.Sprintf("Kernel syscall %q — no reference entry available.", name)
	if got.description != want {
		t.Errorf("syscallInfo(%q).description =\n  %q\nwant:\n  %q", name, got.description, want)
	}
}
