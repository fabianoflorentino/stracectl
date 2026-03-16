package cmd

import (
	"fmt"
	"testing"

	"github.com/fabianoflorentino/stracectl/internal/tracer"
)

func TestRunCmd_SelectTracerError(t *testing.T) {
	old := selectTracer
	defer func() { selectTracer = old }()
	selectTracer = func(backend string) (tracer.Tracer, error) {
		return nil, fmt.Errorf("select failed")
	}

	if err := runCmd.RunE(nil, []string{"true"}); err == nil {
		t.Fatalf("expected error when selectTracer fails")
	}
}
