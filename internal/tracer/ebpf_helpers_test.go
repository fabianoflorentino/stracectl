//go:build ebpf
// +build ebpf

package tracer

import (
	"strings"
	"testing"
)

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

func TestFormatRawArg(t *testing.T) {
	if formatRawArg(0) != "0" {
		t.Fatalf("formatRawArg(0) != 0")
	}
	if formatRawArg(10) != "10" {
		t.Fatalf("formatRawArg(10) != 10")
	}
	if formatRawArg(0x7fffe000) != "0x7fffe000" {
		t.Fatalf("formatRawArg(ptr) hex expected")
	}
}

func TestFormatGenericArgs(t *testing.T) {
	// All-zero args should produce empty string
	var z [6]uint64
	if got := formatGenericArgs(z); got != "" {
		t.Fatalf("formatGenericArgs(all-zero) = %q, want empty", got)
	}

	// Only first arg non-zero
	var a [6]uint64
	a[0] = 3
	got := formatGenericArgs(a)
	if got != "3" {
		t.Fatalf("formatGenericArgs({3,0,...}) = %q, want %q", got, "3")
	}

	// Trailing zeros are stripped
	var b [6]uint64
	b[0] = 1
	b[1] = 2
	b[2] = 0
	got = formatGenericArgs(b)
	if got != "1, 2" {
		t.Fatalf("formatGenericArgs trailing zeros = %q, want %q", got, "1, 2")
	}
}

func TestFormatSyscallArgs_Read(t *testing.T) {
	var args [6]uint64
	args[0] = 5          // fd
	args[1] = 0x7fff0000 // buf pointer
	args[2] = 4096       // count
	got := formatSyscallArgs("read", args, 4096)
	want := "5, 0x7fff0000, 4096"
	if got != want {
		t.Fatalf("read args: got %q, want %q", got, want)
	}
}

func TestFormatSyscallArgs_Open(t *testing.T) {
	var args [6]uint64
	args[0] = 0x7fff1234 // path ptr
	args[1] = 0100 | 02  // O_CREAT | O_RDWR = 0102
	args[2] = 0644       // mode
	got := formatSyscallArgs("open", args, 3)
	if !strings.Contains(got, "O_CREAT") {
		t.Fatalf("open args missing O_CREAT: %q", got)
	}
	if !strings.Contains(got, "O_RDWR") {
		t.Fatalf("open args missing O_RDWR: %q", got)
	}
	// mode octal
	if !strings.Contains(got, "0644") {
		t.Fatalf("open args missing octal mode: %q", got)
	}
}

func TestFormatSyscallArgs_Openat_ATFDCWD(t *testing.T) {
	const AT_FDCWD = ^uint64(99) // -100 as uint64
	var args [6]uint64
	args[0] = AT_FDCWD   // AT_FDCWD
	args[1] = 0x7fff5678 // path ptr
	args[2] = 0          // O_RDONLY
	args[3] = 0
	got := formatSyscallArgs("openat", args, 5)
	if !strings.Contains(got, "AT_FDCWD") {
		t.Fatalf("openat args missing AT_FDCWD: %q", got)
	}
}

func TestFormatSyscallArgs_Mmap(t *testing.T) {
	var args [6]uint64
	args[0] = 0          // addr (NULL)
	args[1] = 4096       // length
	args[2] = 3          // PROT_READ|PROT_WRITE
	args[3] = 0x22       // MAP_PRIVATE|MAP_ANONYMOUS
	args[4] = ^uint64(0) // fd = -1
	args[5] = 0          // offset
	got := formatSyscallArgs("mmap", args, 0x7f000000)
	if !strings.Contains(got, "PROT_READ") {
		t.Fatalf("mmap args missing PROT_READ: %q", got)
	}
	if !strings.Contains(got, "PROT_WRITE") {
		t.Fatalf("mmap args missing PROT_WRITE: %q", got)
	}
	if !strings.Contains(got, "MAP_PRIVATE") {
		t.Fatalf("mmap args missing MAP_PRIVATE: %q", got)
	}
	if !strings.Contains(got, "MAP_ANONYMOUS") {
		t.Fatalf("mmap args missing MAP_ANONYMOUS: %q", got)
	}
}

func TestFormatSyscallArgs_Socket(t *testing.T) {
	var args [6]uint64
	args[0] = 2 // AF_INET
	args[1] = 1 // SOCK_STREAM
	args[2] = 0 // protocol
	got := formatSyscallArgs("socket", args, 4)
	if !strings.Contains(got, "SOCK_STREAM") {
		t.Fatalf("socket args missing SOCK_STREAM: %q", got)
	}
}

func TestFormatSyscallArgs_Socket_Flags(t *testing.T) {
	var args [6]uint64
	args[0] = 2
	args[1] = 1 | 0x800 // SOCK_STREAM | SOCK_NONBLOCK
	args[2] = 0
	got := formatSyscallArgs("socket", args, 4)
	if !strings.Contains(got, "SOCK_NONBLOCK") {
		t.Fatalf("socket args missing SOCK_NONBLOCK: %q", got)
	}
}

func TestFormatSyscallArgs_Clone(t *testing.T) {
	var args [6]uint64
	// CLONE_VM|CLONE_FS|CLONE_FILES|CLONE_SIGHAND|CLONE_THREAD + SIGCHLD(17)
	args[0] = 0x00000100 | 0x00000200 | 0x00000400 | 0x00000800 | 0x00010000 | 17
	got := formatSyscallArgs("clone", args, 12345)
	if !strings.Contains(got, "CLONE_THREAD") {
		t.Fatalf("clone args missing CLONE_THREAD: %q", got)
	}
	if !strings.Contains(got, "CLONE_FILES") {
		t.Fatalf("clone args missing CLONE_FILES: %q", got)
	}
}

func TestFormatSyscallArgs_NoArgs(t *testing.T) {
	var args [6]uint64
	// Syscalls with no arguments should return empty string
	for _, name := range []string{"fork", "vfork", "getpid", "gettid", "sched_yield", "pause", "rt_sigreturn"} {
		got := formatSyscallArgs(name, args, 0)
		if got != "" {
			t.Fatalf("%s: expected empty args, got %q", name, got)
		}
	}
}

func TestOpenFlagsStr(t *testing.T) {
	if got := openFlagsStr(0); got != "O_RDONLY" {
		t.Fatalf("O_RDONLY: got %q", got)
	}
	if got := openFlagsStr(1); !strings.Contains(got, "O_WRONLY") {
		t.Fatalf("O_WRONLY: got %q", got)
	}
	if got := openFlagsStr(0100 | 2); !strings.Contains(got, "O_CREAT") || !strings.Contains(got, "O_RDWR") {
		t.Fatalf("O_CREAT|O_RDWR: got %q", got)
	}
}

func TestMmapProtStr(t *testing.T) {
	if got := mmapProtStr(0); got != "PROT_NONE" {
		t.Fatalf("PROT_NONE: got %q", got)
	}
	if got := mmapProtStr(1 | 2 | 4); got != "PROT_READ|PROT_WRITE|PROT_EXEC" {
		t.Fatalf("all prot flags: got %q", got)
	}
}

func TestAtFdStr(t *testing.T) {
	const AT_FDCWD = ^uint64(99)
	if got := atFdStr(AT_FDCWD); got != "AT_FDCWD" {
		t.Fatalf("AT_FDCWD: got %q", got)
	}
	if got := atFdStr(7); got != "7" {
		t.Fatalf("fd 7: got %q", got)
	}
}
