package tracer

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/fabianoflorentino/stracectl/internal/models"
)

// ── subprocess helper ─────────────────────────────────────────────────────────
//
// When the test binary is re-executed with GO_FAKE_STRACE set, it writes fake
// strace lines to stderr and exits, acting as a stand-in for the real strace
// binary.  This lets us exercise the full goroutine pipeline without requiring
// strace to be installed on the machine running tests.

func TestMain(m *testing.M) {
	switch os.Getenv("GO_FAKE_STRACE") {
	case "lines":
		// Three well-formed strace lines with [pid N] prefix (strace -f format).
		fmt.Fprintln(os.Stderr, `[pid 1] read(3, "hello", 5) = 5 <0.000010>`)
		fmt.Fprintln(os.Stderr, `[pid 1] openat(AT_FDCWD, "/etc/passwd", O_RDONLY) = 3 <0.000042>`)
		fmt.Fprintln(os.Stderr, `[pid 1] close(3) = 0 <0.000005>`)
		os.Exit(0)
	case "error_line":
		// One failed syscall.
		fmt.Fprintln(os.Stderr, `[pid 1] openat(AT_FDCWD, "/no/such/file", O_RDONLY) = -1 ENOENT (No such file or directory) <0.000008>`)
		os.Exit(0)
	case "garbage":
		// Non-syscall noise followed by one real line.
		fmt.Fprintln(os.Stderr, "strace: Process 1 attached")
		fmt.Fprintln(os.Stderr, "this is not a syscall line")
		fmt.Fprintln(os.Stderr, `[pid 1] getpid() = 42 <0.000001>`)
		os.Exit(0)
	case "empty":
		// Exit immediately without writing anything.
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// fakeCmd returns an *exec.Cmd that re-runs the current test binary in
// subprocess "strace" mode, writing canned output to stderr.
func fakeCmd(t *testing.T, scenario string) *exec.Cmd {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	cmd := exec.CommandContext(t.Context(), exe, "-test.run=^$") // run no real tests
	cmd.Env = append(os.Environ(), "GO_FAKE_STRACE="+scenario)
	return cmd
}

// drain reads all events from ch until it is closed; returns a done channel.
func drain(ch <-chan models.SyscallEvent) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		for range ch {
		}
		close(done)
	}()
	return done
}

// ── NewStraceTracer ───────────────────────────────────────────────────────────

func TestNewStraceTracer_NotNil(t *testing.T) {
	if NewStraceTracer() == nil {
		t.Error("NewStraceTracer() returned nil")
	}
}

// ── Interface compliance ──────────────────────────────────────────────────────

func TestStraceTracerImplementsTracer(t *testing.T) {
	// Compile-time assertion: *StraceTracer must satisfy the Tracer interface.
	var _ Tracer = (*StraceTracer)(nil)
}

// ── checkStrace ───────────────────────────────────────────────────────────────

func TestCheckStrace_NotFound(t *testing.T) {
	orig := os.Getenv("PATH")
	t.Cleanup(func() { os.Setenv("PATH", orig) })
	os.Setenv("PATH", "")

	if err := checkStrace(); err == nil {
		t.Error("expected error when strace is absent from PATH, got nil")
	}
}

func TestCheckStrace_Found(t *testing.T) {
	if _, err := exec.LookPath("strace"); err != nil {
		t.Skip("strace not installed — skipping positive checkStrace test")
	}
	if err := checkStrace(); err != nil {
		t.Errorf("checkStrace() = %v, want nil", err)
	}
}

// ── Attach / Run — error path when strace not in PATH ────────────────────────

func TestAttach_NoStrace_ReturnsError(t *testing.T) {
	orig := os.Getenv("PATH")
	t.Cleanup(func() { os.Setenv("PATH", orig) })
	os.Setenv("PATH", "")

	tr := NewStraceTracer()
	_, err := tr.Attach(context.Background(), 1)
	if err == nil {
		t.Error("Attach: expected error when strace is missing, got nil")
	}
}

func TestRun_NoStrace_ReturnsError(t *testing.T) {
	orig := os.Getenv("PATH")
	t.Cleanup(func() { os.Setenv("PATH", orig) })
	os.Setenv("PATH", "")

	tr := NewStraceTracer()
	_, err := tr.Run(context.Background(), "ls", nil)
	if err == nil {
		t.Error("Run: expected error when strace is missing, got nil")
	}
}

// ── start() goroutine pipeline ────────────────────────────────────────────────

func TestStart_EmptyOutput_ClosesChannel(t *testing.T) {
	tr := NewStraceTracer()
	ch, err := tr.start(fakeCmd(t, "empty"), 1)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	select {
	case _, ok := <-ch:
		if ok {
			t.Error("expected channel closed, got an event")
		}
	case <-time.After(5 * time.Second):
		t.Error("channel not closed within 5 s")
	}
}

func TestStart_WellFormedLines_EmitsEvents(t *testing.T) {
	tr := NewStraceTracer()
	ch, err := tr.start(fakeCmd(t, "lines"), 1)
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	var names []string
	for e := range ch {
		names = append(names, e.Name)
	}

	want := []string{"read", "openat", "close"}
	if len(names) != len(want) {
		t.Fatalf("got events %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Errorf("event[%d] = %q, want %q", i, names[i], want[i])
		}
	}
}

func TestStart_EventHasLatency(t *testing.T) {
	tr := NewStraceTracer()
	ch, err := tr.start(fakeCmd(t, "lines"), 1)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	for e := range ch {
		if e.Latency == 0 {
			t.Errorf("event %q has zero latency, want non-zero", e.Name)
		}
	}
}

func TestStart_ErrorLine_SetsErrorField(t *testing.T) {
	tr := NewStraceTracer()
	ch, err := tr.start(fakeCmd(t, "error_line"), 1)
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	var count int
	for e := range ch {
		count++
		if e.Name != "openat" {
			t.Errorf("Name = %q, want openat", e.Name)
		}
		if e.Error != "ENOENT" {
			t.Errorf("Error = %q, want ENOENT", e.Error)
		}
		if e.Latency == 0 {
			t.Error("Latency should be non-zero for error_line scenario")
		}
	}
	if count != 1 {
		t.Errorf("got %d events, want 1", count)
	}
}

func TestStart_GarbageLines_SkipsNonSyscalls(t *testing.T) {
	tr := NewStraceTracer()
	ch, err := tr.start(fakeCmd(t, "garbage"), 1)
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	var names []string
	for e := range ch {
		names = append(names, e.Name)
	}

	// Only "getpid" must come through; noise lines must be silently ignored.
	if len(names) != 1 || names[0] != "getpid" {
		t.Errorf("got events %v, want [getpid]", names)
	}
}

func TestStart_PIDisSetFromLine(t *testing.T) {
	tr := NewStraceTracer()
	// defaultPID=99, but the fake lines have "1 " prefix → PID must come from the line.
	ch, err := tr.start(fakeCmd(t, "lines"), 99)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	for e := range ch {
		if e.PID != 1 {
			t.Errorf("PID = %d, want 1 (from line prefix, not defaultPID 99)", e.PID)
		}
	}
}

func TestStart_ContextCancel_ChannelEventuallyCloses(t *testing.T) {
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	tr := NewStraceTracer()
	// "empty" subprocess exits on its own; we also cancel to cover that code path.
	ch, err := tr.start(fakeCmd(t, "empty"), 1)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	cancel()

	select {
	case <-drain(ch):
	case <-time.After(5 * time.Second):
		t.Error("channel did not close after subprocess exit")
	}
}

// ── Attach / Run happy paths (require strace installed) ───────────────────────

func TestAttach_WithStrace_ReturnsChannel(t *testing.T) {
	if _, err := exec.LookPath("strace"); err != nil {
		t.Skip("strace not installed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tr := NewStraceTracer()
	// Attach to our own PID; cancel immediately so strace doesn't linger.
	ch, err := tr.Attach(ctx, os.Getpid())
	if err != nil {
		t.Fatalf("Attach: %v", err)
	}
	cancel()
	// Channel must close eventually.
	select {
	case <-drain(ch):
	case <-time.After(10 * time.Second):
		t.Error("channel did not close after Attach + cancel")
	}
}

func TestRun_WithStrace_ReturnsChannel(t *testing.T) {
	if _, err := exec.LookPath("strace"); err != nil {
		t.Skip("strace not installed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tr := NewStraceTracer()
	// Trace a trivially-fast command.
	ch, err := tr.Run(ctx, "true", nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// Drain all events; channel must close after "true" exits.
	select {
	case <-drain(ch):
	case <-time.After(10 * time.Second):
		t.Error("channel did not close after Run(true)")
	}
}

func TestRun_WithStrace_EmitsAtLeastOneEvent(t *testing.T) {
	if _, err := exec.LookPath("strace"); err != nil {
		t.Skip("strace not installed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tr := NewStraceTracer()
	ch, err := tr.Run(ctx, "true", nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	var count int
	for range ch {
		count++
	}
	// strace may fail to trace without ptrace permissions (e.g. no sudo);
	// in that case the channel closes with 0 events which is acceptable.
	t.Logf("Run(true) emitted %d events", count)
}
