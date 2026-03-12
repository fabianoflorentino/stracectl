//go:build !ebpf
// +build !ebpf

package tracer

import (
	"context"
	"fmt"

	"github.com/fabianoflorentino/stracectl/internal/models"
)

// EBPFTracer stub for builds without the `ebpf` tag. This allows the
// package to compile when bpf2go-generated artifacts are not present.
type EBPFTracer struct{}

func NewEBPFTracer() *EBPFTracer { return &EBPFTracer{} }

func (t *EBPFTracer) Attach(ctx context.Context, pid int) (<-chan models.SyscallEvent, error) {
	return nil, fmt.Errorf("eBPF tracer not available in this build: build with -tags=ebpf to enable")
}

func (t *EBPFTracer) Run(ctx context.Context, program string, args []string) (<-chan models.SyscallEvent, error) {
	return nil, fmt.Errorf("eBPF tracer not available in this build: build with -tags=ebpf to enable")
}
