//go:build !ebpf

package tracer

import (
	"context"
	"testing"
)

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
