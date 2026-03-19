package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/fabianoflorentino/stracectl/internal/aggregator"
	"github.com/fabianoflorentino/stracectl/internal/models"
	"github.com/fabianoflorentino/stracectl/internal/report"
	"github.com/fabianoflorentino/stracectl/internal/server"
	"github.com/fabianoflorentino/stracectl/internal/tracer"
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
// runTrace now accepts wsToken for WebSocket authentication
// runTrace wires events from the tracer into the aggregator, then either
// starts the HTTP server (if serveAddr is non-empty) or the TUI. It blocks
// until the context is cancelled or the UI/server returns, and it drains the
// event channel before returning.
//
// We start the UI first (in TUI mode) to allow BubbleTea to initialize the
// terminal state and receive the initial WindowSizeMsg before the tracer
// spawns subprocesses that may affect terminal/process-group state. The
// tracer is started shortly after the UI is up.
func runTrace(ctx context.Context, tracerCtx context.Context, cancelTracer context.CancelFunc, tr tracer.Tracer, program string, args []string, agg *aggregator.Aggregator, serveAddr, wsToken, reportPath string, reportTopFiles int, label string) error {
	// done is closed when the events channel drains (traced process exited).
	done := make(chan struct{})

	var wg sync.WaitGroup

	var runErr error

	if serveAddr != "" {
		// Server mode: start tracer first (so the dashboard has data immediately),
		// then start HTTP server in the foreground.
		events, err := tr.Run(tracerCtx, program, args)
		if err != nil {
			// If tracer failed to start, surface the error.
			if runTryElevate && os.Getenv("STRACECTL_TRIED_ELEVATE") != "1" {
				fmt.Fprintln(os.Stderr, "eBPF failed to load; attempting to re-run with elevated memlock via sudo/prlimit...")
				tryElevateAndRerun()
				fmt.Fprintln(os.Stderr, "elevation attempt failed; continuing with original error")
			}
			return err
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			defer close(done)
			for event := range events {
				agg.Add(event)
			}
			agg.SetDone()
		}()

		fmt.Fprintf(os.Stderr, "serving on %s\n", serveAddr)
		srv := server.New(serveAddr, agg, wsToken)
		runErr = srv.Start(ctx)
	} else {
		// TUI mode: start UI first in a goroutine so it can initialize the
		// terminal. We then start the tracer. The UI runs in a goroutine so
		// runTrace can manage the tracer lifecycle; we wait for the UI to
		// return and then cancel the tracer.
		uiErrCh := make(chan error, 1)
		wg.Add(1)
		go func() {
			defer wg.Done()
			uiErrCh <- ui.Run(agg, label, done)
		}()

		// Allow the UI a short time to initialize and produce a window-size
		// event. We also poll the UI debug event log (if present) for a
		// WindowSizeMsg signal to avoid races on various terminals.
		waitForUIReady(500 * time.Millisecond)

		// Start tracer and drain events into the aggregator.
		events, err := tr.Run(tracerCtx, program, args)
		if err != nil {
			if runTryElevate && os.Getenv("STRACECTL_TRIED_ELEVATE") != "1" {
				fmt.Fprintln(os.Stderr, "eBPF failed to load; attempting to re-run with elevated memlock via sudo/prlimit...")
				tryElevateAndRerun()
				fmt.Fprintln(os.Stderr, "elevation attempt failed; continuing with original error")
			}
			// If the tracer fails to start, stop the UI and return the error.
			cancelTracer()
			// Ensure UI goroutine has had a chance to exit.
			runErr = <-uiErrCh
			if runErr == nil {
				runErr = err
			}
			return runErr
		}

		// runTraceWithEvents compatibility logic moved to top-level; continue
		// draining events below.

		wg.Add(1)
		go func() {
			defer wg.Done()
			defer close(done)
			for event := range events {
				agg.Add(event)
			}
			agg.SetDone()
		}()

		// Wait for UI to exit; runErr receives the ui.Run result.
		runErr = <-uiErrCh
	}

	// Kill the tracer subprocess now that the UI/server has stopped. This
	// closes the events channel so the consumer goroutine can finish.
	cancelTracer()
	wg.Wait()

	if runErr == nil && reportPath != "" {
		if err := writeHTMLReport(reportPath, agg, label, reportTopFiles); err != nil {
			return err
		}
	}

	return runErr
}

// waitForUIReady polls the UI events debug log for a window-size event until
// the timeout expires. This is a best-effort diagnostic helper used to avoid
// race conditions when the UI and tracer initialize concurrently.
func waitForUIReady(maxWait time.Duration) {
	deadline := time.Now().Add(maxWait)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile("/tmp/stracectl_ui_events.log")
		if err == nil {
			if strings.Contains(string(data), "ev=window-size") {
				return
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
}

// writeHTMLReport writes a self-contained HTML report to path.
// Any error is returned to the caller and propagates as a non-zero exit code.
func writeHTMLReport(path string, agg *aggregator.Aggregator, label string, topFilesLimit int) error {
	if err := report.Write(path, agg, label, topFilesLimit); err != nil {
		return fmt.Errorf("writing report %s: %w", path, err)
	}
	fmt.Fprintf(os.Stderr, "report written to %s\n", path)
	return nil
}

// runTraceWithEvents is a compatibility wrapper for callers that already have
// an events channel (e.g. attach mode). It reuses the previous draining
// behavior: start a consumer goroutine to feed the aggregator then start the
// UI or server. Kept at top-level so other commands (attach) can call it.
func runTraceWithEvents(ctx context.Context, cancelTracer context.CancelFunc, events <-chan models.SyscallEvent, agg *aggregator.Aggregator, serveAddr, wsToken, reportPath string, reportTopFiles int, label string) error {
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(done)
		for event := range events {
			agg.Add(event)
		}
		agg.SetDone()
	}()

	var runErr error
	if serveAddr != "" {
		fmt.Fprintf(os.Stderr, "serving on %s\n", serveAddr)
		srv := server.New(serveAddr, agg, wsToken)
		runErr = srv.Start(ctx)
	} else {
		runErr = ui.Run(agg, label, done)
	}

	cancelTracer()
	wg.Wait()

	if runErr == nil && reportPath != "" {
		if err := writeHTMLReport(reportPath, agg, label, reportTopFiles); err != nil {
			return err
		}
	}

	return runErr
}
