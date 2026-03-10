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
// runTrace agora aceita wsToken para autenticação WebSocket
func runTrace(ctx context.Context, cancelTracer context.CancelFunc, events <-chan models.SyscallEvent, agg *aggregator.Aggregator, serveAddr, wsToken, reportPath, label string) error {
	// done is closed when the events channel drains (traced process exited).
	done := make(chan struct{})

	var wg sync.WaitGroup
	wg.Go(func() {
		defer close(done)
		for event := range events {
			agg.Add(event)
		}
		// Mark the traced process as done so the server can notify clients.
		agg.SetDone()
	})

	var runErr error
	if serveAddr != "" {
		fmt.Fprintf(os.Stderr, "serving on %s\n", serveAddr)
		srv := server.New(serveAddr, agg, wsToken)
		runErr = srv.Start(ctx)
	} else {
		runErr = ui.Run(agg, label, done)
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
// Any error is returned to the caller and propagates as a non-zero exit code.
func writeHTMLReport(path string, agg *aggregator.Aggregator, label string) error {
	if err := report.Write(path, agg, label); err != nil {
		return fmt.Errorf("writing report %s: %w", path, err)
	}
	fmt.Fprintf(os.Stderr, "report written to %s\n", path)
	return nil
}
