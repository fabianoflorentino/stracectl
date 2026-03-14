//go:build !ebpf
// +build !ebpf

package tracer

import (
	"context"
	"fmt"

	"github.com/fabianoflorentino/stracectl/internal/models"
)

// ebpfBuild is false in non-ebpf builds. See ebpf.go for the true variant.
var ebpfBuild = false

// EBPFTracer stub for builds without the `ebpf` tag. This allows the
// package to compile when bpf2go-generated artifacts are not present.
type EBPFTracer struct{}

func NewEBPFTracer() *EBPFTracer { return &EBPFTracer{} }

// SetForce is a no-op for non-ebpf builds.
func (t *EBPFTracer) SetForce(v bool) {}

// SetUnfiltered is a no-op for non-ebpf builds.
func (t *EBPFTracer) SetUnfiltered(v bool) {}

func (t *EBPFTracer) Attach(ctx context.Context, pid int) (<-chan models.SyscallEvent, error) {
	return nil, fmt.Errorf("eBPF tracer not available in this build: build with -tags=ebpf to enable")
}

func (t *EBPFTracer) Run(ctx context.Context, program string, args []string) (<-chan models.SyscallEvent, error) {
	return nil, fmt.Errorf("eBPF tracer not available in this build: build with -tags=ebpf to enable")
}
