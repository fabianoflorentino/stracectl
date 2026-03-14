package cmd

import (
	"context"
	"testing"

	"github.com/fabianoflorentino/stracectl/internal/models"
)

// fakeTracer implements the Tracer interface and the optional configuration
// setters so we can verify applyEBPFOptions invokes them correctly.
type fakeTracer struct {
	setForceCalled      bool
	setUnfilteredCalled bool
	forceVal            bool
	unfilteredVal       bool
}

func (f *fakeTracer) Attach(ctx context.Context, pid int) (<-chan models.SyscallEvent, error) {
	return nil, nil
}
func (f *fakeTracer) Run(ctx context.Context, program string, args []string) (<-chan models.SyscallEvent, error) {
	return nil, nil
}

func (f *fakeTracer) SetForce(v bool)      { f.setForceCalled = true; f.forceVal = v }
func (f *fakeTracer) SetUnfiltered(v bool) { f.setUnfilteredCalled = true; f.unfilteredVal = v }

func TestApplyEBPFOptions_ConfiguresTracer(t *testing.T) {
	f := &fakeTracer{}
	applyEBPFOptions(f, true, false)
	if !f.setForceCalled || f.forceVal != true {
		t.Fatalf("SetForce not invoked or wrong value: called=%v val=%v", f.setForceCalled, f.forceVal)
	}
	if !f.setUnfilteredCalled || f.unfilteredVal != false {
		t.Fatalf("SetUnfiltered not invoked or wrong value: called=%v val=%v", f.setUnfilteredCalled, f.unfilteredVal)
	}
}
