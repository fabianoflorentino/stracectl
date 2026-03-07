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
func runTrace(ctx context.Context, events <-chan models.SyscallEvent, agg *aggregator.Aggregator, serveAddr, label string) error {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for event := range events {
			agg.Add(event)
		}
	}()

	var runErr error
	if serveAddr != "" {
		fmt.Fprintf(os.Stderr, "serving on %s\n", serveAddr)
		srv := server.New(serveAddr, agg)
		runErr = srv.Start(ctx)
	} else {
		runErr = ui.Run(agg, label)
	}

	// Wait for the consumer goroutine to finish draining the event channel
	// before the caller returns and any deferred teardown runs.
	wg.Wait()
	return runErr
}
