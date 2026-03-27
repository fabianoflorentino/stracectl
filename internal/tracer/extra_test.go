package tracer

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"testing"

	"golang.org/x/sys/unix"
)

// Test explicit Select branches and unknown backend error.
func TestSelect_ExplicitBackendsAndUnknown(t *testing.T) {
	if tr, err := Select("ebpf"); err != nil {
		t.Fatalf("Select(ebpf) returned error: %v", err)
	} else {
		// In non-ebpf builds this is the stub type; ensure non-nil.
		if tr == nil {
			t.Fatalf("Select(ebpf) returned nil tracer")
		}
	}

	if tr, err := Select("strace"); err != nil {
		t.Fatalf("Select(strace) returned error: %v", err)
	} else {
		if tr == nil {
			t.Fatalf("Select(strace) returned nil tracer")
		}
	}

	if _, err := Select("nope"); err == nil {
		t.Fatalf("Select(unknown) expected error, got nil")
	}
}

// Test that the EBPF stub behaves as expected in non-ebpf builds.
func TestEBPFStub_NoEBPF(t *testing.T) {
	tr := NewEBPFTracer()
	tr.SetForce(true)      // no-op
	tr.SetUnfiltered(true) // no-op

	if ch, err := tr.Attach(context.Background(), 1); err == nil {
		if ch != nil {
			t.Fatalf("expected Attach to return error when ebpf disabled, got channel")
		}
	}

	if ch, err := tr.Run(context.Background(), "true", nil); err == nil {
		if ch != nil {
			t.Fatalf("expected Run to return error when ebpf disabled, got channel")
		}
	}
}

type closeOK struct{}

func (c *closeOK) Close() error { return nil }

type closeErr struct{}

func (c *closeErr) Close() error { return errors.New("close failed") }

func Test_EbpfClose_Helper(t *testing.T) {
	// Should return nil when all closers succeed
	if err := _EbpfClose(&closeOK{}); err != nil {
		t.Fatalf("_EbpfClose returned unexpected error: %v", err)
	}

	// Should return an error if any closer fails
	if err := _EbpfClose(&closeOK{}, &closeErr{}); err == nil {
		t.Fatalf("_EbpfClose expected error, got nil")
	}
}

func TestStart_EAGAINEmpty_LogsDebug(t *testing.T) {
	tr := NewStraceTracer()
	// Enable debug to hit the noisy logging branch.
	Debug = true
	defer func() { Debug = false }()

	ch, err := tr.start(fakeCmd(t, "eagain_empty"), 1)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	// Drain channel until closed to ensure goroutine runs to completion.
	for range ch {
	}
}

func TestEbpfAvailable_UnameErrorAndBadParse(t *testing.T) {
	// Backup and restore globals
	origUname := unameFunc
	origBuild := ebpfBuild
	t.Cleanup(func() {
		unameFunc = origUname
		ebpfBuild = origBuild
	})

	ebpfBuild = true
	// Simulate uname failing
	unameFunc = func(u *unix.Utsname) error { return fmt.Errorf("fail uname") }
	if ebpfAvailable() {
		t.Fatal("expected ebpfAvailable=false when uname returns error")
	}

	// Simulate unparsable release string
	unameFunc = func(u *unix.Utsname) error {
		writeRelease(u, "not-a-version")
		return nil
	}
	if ebpfAvailable() {
		t.Fatal("expected ebpfAvailable=false for unparsable release")
	}
}

func TestSelect_Auto_NotRootFallsBack(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("requires linux")
	}
	origUname := unameFunc
	origBuild := ebpfBuild
	origEuid := getEuid
	t.Cleanup(func() {
		unameFunc = origUname
		ebpfBuild = origBuild
		getEuid = origEuid
	})

	ebpfBuild = true
	unameFunc = func(u *unix.Utsname) error {
		writeRelease(u, "5.8.0")
		return nil
	}
	// Simulate non-root
	getEuid = func() int { return 1000 }

	tr, err := Select("auto")
	if err != nil {
		t.Fatalf("Select(auto) returned error: %v", err)
	}
	if _, ok := tr.(*StraceTracer); !ok {
		t.Fatalf("Select(auto) returned %T for non-root, want *StraceTracer", tr)
	}
}
