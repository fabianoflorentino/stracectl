package cmd

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/fabianoflorentino/stracectl/internal/aggregator"
	"github.com/fabianoflorentino/stracectl/internal/models"
	"github.com/fabianoflorentino/stracectl/internal/report"
	"github.com/fabianoflorentino/stracectl/internal/server"
	"github.com/fabianoflorentino/stracectl/internal/ui"
)

// runTrace wires events from the tracer into the aggregator, then either
// starts the HTTP server (if serveAddr is non-empty) or the TUI. It blocks
// until the context is cancelled or the UI/server returns, and it drains the
// event channel before returning.
//
// cancelTracer must be the cancel function for the context that was passed to
// Tracer.Attach / Tracer.Run. Calling it kills the strace subprocess so the
// events channel closes promptly after the UI or server has exited.
func runTrace(ctx context.Context, cancelTracer context.CancelFunc, events <-chan models.SyscallEvent, agg *aggregator.Aggregator, serveAddr, reportPath, label string) error {
	var wg sync.WaitGroup
	wg.Go(func() {
		for event := range events {
			agg.Add(event)
		}
	})

	var runErr error
	if serveAddr != "" {
		fmt.Fprintf(os.Stderr, "serving on %s\n", serveAddr)
		srv := server.New(serveAddr, agg)
		runErr = srv.Start(ctx)
	} else {
		runErr = ui.Run(agg, label)
	}

	// Kill the strace subprocess now that the UI/server has stopped.
	// This closes the events channel so the consumer goroutine can finish.
	cancelTracer()
	wg.Wait()

	if runErr == nil && reportPath != "" {
		if err := writeHTMLReport(reportPath, agg, label); err != nil {
			return err
		}
	}

	return runErr
}

// writeHTMLReport writes a self-contained HTML report to path.
// Errors are printed to stderr but do not affect the exit code.
func writeHTMLReport(path string, agg *aggregator.Aggregator, label string) error {
	if err := report.Write(path, agg, label); err != nil {
		return fmt.Errorf("writing report %s: %w", path, err)
	}
	fmt.Fprintf(os.Stderr, "report written to %s\n", path)
	return nil
}
