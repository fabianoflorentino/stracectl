package ui

import (
	"fmt"
	"testing"

	uirender "github.com/fabianoflorentino/stracectl/internal/ui/render"
)

// TestSyscallAliasesPointToValidCanonicals verifies that every entry in
// SyscallAliases resolves to a key that actually exists in SyscallDetails.
func TestSyscallAliasesPointToValidCanonicals(t *testing.T) {
	for alias, canonical := range uirender.SyscallAliases {
		if _, ok := uirender.SyscallDetails[canonical]; !ok {
			t.Errorf("SyscallAliases[%q] = %q, but %q has no entry in SyscallDetails", alias, canonical, canonical)
		}
	}
}

// TestSyscallInfoKnownNames verifies that SyscallInfo returns the expected
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
			got := uirender.SyscallInfo(tc.input)
			if got.Description != tc.wantDesc {
				t.Errorf("SyscallInfo(%q).Description =\n  %q\nwant:\n  %q", tc.input, got.Description, tc.wantDesc)
			}
		})
	}
}

// TestSyscallInfoUnknownName verifies that an unrecognised syscall returns a
// non-empty generic description instead of a zero value.
func TestSyscallInfoUnknownName(t *testing.T) {
	name := "totally_unknown_syscall_xyz"
	got := uirender.SyscallInfo(name)
	if got.Description == "" {
		t.Errorf("SyscallInfo(%q).Description is empty; expected a generic fallback", name)
	}
	want := fmt.Sprintf("Kernel syscall %q — no reference entry available.", name)
	if got.Description != want {
		t.Errorf("SyscallInfo(%q).Description =\n  %q\nwant:\n  %q", name, got.Description, want)
	}
}
