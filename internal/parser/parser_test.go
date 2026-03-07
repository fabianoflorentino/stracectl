package parser_test

import (
	"testing"
	"time"

	"github.com/fabianoflorentino/stracectl/internal/parser"
)

// ── table-driven tests ────────────────────────────────────────────────────────

func TestParse(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		line       string
		defaultPID int
		// expected
		wantNil    bool // Parse should return nil, nil
		wantName   string
		wantPID    int
		wantRetVal string
		wantError  string // empty = success
		wantArgs   string
		wantMinLat time.Duration // latency must be >= this
	}{
		{
			name:       "simple syscall no latency",
			line:       `read(3, "hello", 128) = 5`,
			defaultPID: 42,
			wantName:   "read",
			wantPID:    42,
			wantRetVal: "5",
			wantArgs:   `3, "hello", 128`,
		},
		{
			name:       "syscall with latency",
			line:       `openat(AT_FDCWD, "/etc/ld.so.cache", O_RDONLY|O_CLOEXEC) = 3 <0.000123>`,
			defaultPID: 1,
			wantName:   "openat",
			wantPID:    1,
			wantRetVal: "3",
			wantMinLat: 100 * time.Microsecond, // 0.000123 s ≈ 123 µs
		},
		{
			name:       "error syscall with errno and latency",
			line:       `openat(AT_FDCWD, "/no/such/file", O_RDONLY) = -1 ENOENT (No such file or directory) <0.000045>`,
			defaultPID: 1,
			wantName:   "openat",
			wantRetVal: "-1",
			wantError:  "ENOENT",
			wantMinLat: 40 * time.Microsecond,
		},
		{
			name:       "prefixed with [pid N]",
			line:       `[pid 1234] write(1, "ok\n", 3) = 3 <0.000010>`,
			defaultPID: 0,
			wantName:   "write",
			wantPID:    1234,
			wantRetVal: "3",
		},
		{
			name:       "unfinished stub is skipped",
			line:       `read(3, <unfinished ...>`,
			defaultPID: 1,
			wantNil:    true,
		},
		{
			name:       "blank line is skipped",
			line:       ``,
			defaultPID: 1,
			wantNil:    true,
		},
		{
			name:       "signal line is skipped",
			line:       `--- SIGCHLD {si_signo=SIGCHLD, si_code=CLD_EXITED} ---`,
			defaultPID: 1,
			wantNil:    true,
		},
		{
			name:       "exit_group",
			line:       `exit_group(0) = ?`,
			defaultPID: 1,
			wantNil:    true, // no return value matched by retRe
		},
		{
			name:       "connect with error",
			line:       `connect(5, {sa_family=AF_INET, sin_port=htons(80), sin_addr=inet_addr("1.2.3.4")}, 16) = -1 ECONNREFUSED (Connection refused) <0.001500>`,
			defaultPID: 99,
			wantName:   "connect",
			wantPID:    99,
			wantRetVal: "-1",
			wantError:  "ECONNREFUSED",
			wantMinLat: 1 * time.Millisecond,
		},
		{
			name:       "mmap call",
			line:       `mmap(NULL, 4096, PROT_READ|PROT_WRITE, MAP_PRIVATE|MAP_ANONYMOUS, -1, 0) = 0x7f1234560000 <0.000008>`,
			defaultPID: 1,
			wantNil:    true, // retval is a hex address, not matched by retRe (-?\d+)
		},
	}

	for _, tc := range cases {
		tc := tc // capture range variable
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := parser.Parse(tc.line, tc.defaultPID)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tc.wantNil {
				if got != nil {
					t.Fatalf("expected nil, got %+v", got)
				}
				return
			}

			if got == nil {
				t.Fatal("expected non-nil event, got nil")
			}

			if got.Name != tc.wantName {
				t.Errorf("Name: want %q, got %q", tc.wantName, got.Name)
			}
			if tc.wantPID != 0 && got.PID != tc.wantPID {
				t.Errorf("PID: want %d, got %d", tc.wantPID, got.PID)
			}
			if tc.wantRetVal != "" && got.RetVal != tc.wantRetVal {
				t.Errorf("RetVal: want %q, got %q", tc.wantRetVal, got.RetVal)
			}
			if got.Error != tc.wantError {
				t.Errorf("Error: want %q, got %q", tc.wantError, got.Error)
			}
			if tc.wantArgs != "" && got.Args != tc.wantArgs {
				t.Errorf("Args: want %q, got %q", tc.wantArgs, got.Args)
			}
			if tc.wantMinLat > 0 && got.Latency < tc.wantMinLat {
				t.Errorf("Latency: want >= %v, got %v", tc.wantMinLat, got.Latency)
			}
		})
	}
}
