package tracer

import (
	"context"
	"errors"
	"testing"
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
