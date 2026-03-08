package cmd

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/fabianoflorentino/stracectl/internal/aggregator"
	"github.com/fabianoflorentino/stracectl/internal/models"
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
func runTrace(ctx context.Context, cancelTracer context.CancelFunc, events <-chan models.SyscallEvent, agg *aggregator.Aggregator, serveAddr, label string) error {
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

	return runErr
}
