package cmd

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fabianoflorentino/stracectl/internal/aggregator"
	"github.com/fabianoflorentino/stracectl/internal/models"
	"github.com/fabianoflorentino/stracectl/internal/report"
	"github.com/fabianoflorentino/stracectl/internal/server"
	"github.com/fabianoflorentino/stracectl/internal/tracer"
	"github.com/fabianoflorentino/stracectl/internal/ui"

	p "github.com/fabianoflorentino/stracectl/internal/privacy"
	paudit "github.com/fabianoflorentino/stracectl/internal/privacy/audit"
	pfilters "github.com/fabianoflorentino/stracectl/internal/privacy/filters"
	pformat "github.com/fabianoflorentino/stracectl/internal/privacy/formatter"
	pout "github.com/fabianoflorentino/stracectl/internal/privacy/output"
	ppipeline "github.com/fabianoflorentino/stracectl/internal/privacy/pipeline"
	predact "github.com/fabianoflorentino/stracectl/internal/privacy/redactor"
	"golang.org/x/term"
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

	startTime := time.Now()

	// Initialize privacy pipeline components if a privacy log path was provided.
	var (
		pEnabled    = privacyLogPath != ""
		pFilter     *pfilters.FilterSet
		pRedactor   p.Redactor
		pFormatter  p.Formatter
		pOutput     p.Output
		auditLogger *paudit.Logger
		eventCount  int
		ttl         time.Duration
	)
	if privacyTTL != "" {
		if d, err := time.ParseDuration(privacyTTL); err == nil {
			ttl = d
		} else {
			fmt.Fprintf(os.Stderr, "warning: invalid privacy-ttl %q: %v; ignoring TTL\n", privacyTTL, err)
		}
	}
	// If user requested full capture, require explicit confirmation in
	// interactive mode or `--force` in non-interactive flows.
	if privacyFull {
		if !privacyForce {
			if term.IsTerminal(syscall.Stdin) {
				fmt.Fprintln(os.Stderr, "\n⚠ WARNING: --full enables full payload capture and may expose sensitive data")
				fmt.Fprint(os.Stderr, "Continue? (y/N): ")
				r := bufio.NewReader(os.Stdin)
				ans, _ := r.ReadString('\n')
				ans = strings.TrimSpace(strings.ToLower(ans))
				if ans != "y" && ans != "yes" {
					fmt.Fprintln(os.Stderr, "--full not enabled; proceeding with safer defaults")
					privacyFull = false
				}
			} else {
				fmt.Fprintln(os.Stderr, "error: --full requires --force in non-interactive contexts; ignoring --full")
				privacyFull = false
			}
		}
	}

	// Derive privacy semantics from privacy-level and flags.
	derivedNoArgs := privacyNoArgs
	if privacyPrivacyLevel == "high" {
		derivedNoArgs = true
	}
	derivedAllowFull := privacyFull || privacyPrivacyLevel == "low"

	if pEnabled {
		pFilter = pfilters.New(privacySyscalls, privacyExclude, nil, nil)

		// Build redactor config
		patterns := []string{}
		if privacyRedactPatterns != "" {
			for _, s := range strings.Split(privacyRedactPatterns, ",") {
				s = strings.TrimSpace(s)
				if s != "" {
					patterns = append(patterns, s)
				}
			}
		}
		rcfg := predact.Config{NoArgs: derivedNoArgs, MaxArgSize: privacyMaxArgSize, Patterns: patterns}
		r, err := predact.New(rcfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to initialize redactor: %v; disabling privacy logging\n", err)
			pEnabled = false
		} else {
			pRedactor = r
		}

		pFormatter = pformat.NewJSONFormatter()

		if pEnabled {
			if privacyLogPath == "stdout" {
				pOutput = pout.NewStdout()
			} else {
				of, err := pout.NewFile(privacyLogPath, ttl, ctx)
				if err != nil {
					fmt.Fprintf(os.Stderr, "warning: cannot open privacy log %s: %v; disabling privacy logging\n", privacyLogPath, err)
					pEnabled = false
				} else {
					pOutput = of
				}
			}
		}

		// Initialize audit logger if privacy logging enabled and output created.
		if pEnabled && pOutput != nil {
			if privacyLogPath == "stdout" {
				// no audit file for stdout
			} else {
				al, err := paudit.New(privacyLogPath + ".audit")
				if err != nil {
					fmt.Fprintf(os.Stderr, "warning: cannot create audit log: %v\n", err)
				} else {
					auditLogger = al
					// Log initial entry
					if err := auditLogger.Log(paudit.Entry{
						"action": "trace_start",
						"label":  label,
						"privacy_opts": map[string]interface{}{
							"no_args":       privacyNoArgs,
							"max_arg_size":  privacyMaxArgSize,
							"syscalls":      privacySyscalls,
							"exclude":       privacyExclude,
							"privacy_level": privacyPrivacyLevel,
							"full":          privacyFull,
							"ttl":           privacyTTL,
						},
						"program": program,
						"args":    strings.Join(args, " "),
					}); err != nil {
						fmt.Fprintf(os.Stderr, "warning: audit log write failed: %v\n", err)
					}
				}
			}
		}
	}

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

		wg.Go(func() {
			defer close(done)
			for event := range events {
				agg.Add(event)
				if pEnabled && pOutput != nil {
					te := p.NewTraceEventFromModel(event)
					if derivedAllowFull {
						te.Args = []p.Arg{{Name: "raw", Value: []byte(event.Args)}}
					}
					if err := ppipeline.Process(&te, pFilter, pRedactor, pFormatter, pOutput); err != nil {
						fmt.Fprintf(os.Stderr, "warning: privacy pipeline error: %v\n", err)
					} else {
						// separate JSON objects with newline
						if err := pOutput.Write([]byte("\n")); err != nil {
							fmt.Fprintf(os.Stderr, "warning: privacy log separator write failed: %v\n", err)
						}
						eventCount++
					}
				}
			}
			agg.SetDone()
		})

		fmt.Fprintf(os.Stderr, "serving on %s\n", serveAddr)
		srv := server.New(serveAddr, agg, wsToken)
		runErr = srv.Start(ctx)
	} else {
		// TUI mode: start UI first in a goroutine so it can initialize the
		// terminal. We then start the tracer. The UI runs in a goroutine so
		// runTrace can manage the tracer lifecycle; we wait for the UI to
		// return and then cancel the tracer.
		uiErrCh := make(chan error, 1)
		readyCh := make(chan struct{})
		wg.Go(func() {
			uiErrCh <- ui.Run(agg, label, done, readyCh)
		})

		// Allow the UI a short time to initialize and produce a window-size
		// event. Fall back after timeout if the event never arrives.
		select {
		case <-readyCh:
		case <-time.After(500 * time.Millisecond):
		}

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

		wg.Go(func() {
			defer close(done)
			for event := range events {
				agg.Add(event)
				if pEnabled && pOutput != nil {
					te := p.NewTraceEventFromModel(event)
					if derivedAllowFull {
						te.Args = []p.Arg{{Name: "raw", Value: []byte(event.Args)}}
					}
					if err := ppipeline.Process(&te, pFilter, pRedactor, pFormatter, pOutput); err != nil {
						fmt.Fprintf(os.Stderr, "warning: privacy pipeline error: %v\n", err)
					} else {
						if err := pOutput.Write([]byte("\n")); err != nil {
							fmt.Fprintf(os.Stderr, "warning: privacy log separator write failed: %v\n", err)
						}
						eventCount++
					}
				}
			}
			agg.SetDone()
		})

		// Wait for UI to exit; runErr receives the ui.Run result.
		runErr = <-uiErrCh
	}

	// Kill the tracer subprocess now that the UI/server has stopped. This
	// closes the events channel so the consumer goroutine can finish.
	cancelTracer()
	wg.Wait()

	// If privacy output is enabled, ensure we flush/close and record audit end
	// entry with event count and SHA256 of the final file (if applicable).
	if pEnabled && pOutput != nil {
		// close to flush writes before hashing
		if err := pOutput.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to close privacy log: %v\n", err)
		}
		if auditLogger != nil && privacyLogPath != "stdout" {
			// compute SHA256 of the privacy log file
			f, err := os.Open(privacyLogPath)
			if err == nil {
				hasher := sha256.New()
				if _, err := io.Copy(hasher, f); err == nil {
					sum := hasher.Sum(nil)
					if err := auditLogger.Log(paudit.Entry{
						"action":      "trace_end",
						"label":       label,
						"event_count": eventCount,
						"file_hash":   hex.EncodeToString(sum),
						"duration":    time.Since(startTime).String(),
					}); err != nil {
						fmt.Fprintf(os.Stderr, "warning: audit log write failed: %v\n", err)
					}
				}
				if err := f.Close(); err != nil {
					fmt.Fprintf(os.Stderr, "warning: failed to close privacy log after hashing: %v\n", err)
				}
			}
		}
	}

	if runErr == nil && reportPath != "" {
		if err := writeHTMLReport(reportPath, agg, label, reportTopFiles); err != nil {
			return err
		}
	}

	return runErr
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
	startTime := time.Now()
	// Initialize privacy pipeline components if a privacy log path was provided.
	var (
		pEnabled    = privacyLogPath != ""
		pFilter     *pfilters.FilterSet
		pRedactor   p.Redactor
		pFormatter  p.Formatter
		pOutput     p.Output
		auditLogger *paudit.Logger
		eventCount  int
		ttl         time.Duration
	)
	if privacyTTL != "" {
		if d, err := time.ParseDuration(privacyTTL); err == nil {
			ttl = d
		}
	}
	// Confirmation and level-derived semantics for compatibility wrapper.
	if privacyFull {
		if !privacyForce {
			if term.IsTerminal(syscall.Stdin) {
				fmt.Fprintln(os.Stderr, "\n⚠ WARNING: --full enables full payload capture and may expose sensitive data")
				fmt.Fprint(os.Stderr, "Continue? (y/N): ")
				r := bufio.NewReader(os.Stdin)
				ans, _ := r.ReadString('\n')
				ans = strings.TrimSpace(strings.ToLower(ans))
				if ans != "y" && ans != "yes" {
					fmt.Fprintln(os.Stderr, "--full not enabled; proceeding with safer defaults")
					privacyFull = false
				}
			} else {
				fmt.Fprintln(os.Stderr, "error: --full requires --force in non-interactive contexts; ignoring --full")
				privacyFull = false
			}
		}
	}

	derivedNoArgs := privacyNoArgs
	if privacyPrivacyLevel == "high" {
		derivedNoArgs = true
	}
	derivedAllowFull := privacyFull || privacyPrivacyLevel == "low"

	if pEnabled {
		pFilter = pfilters.New(privacySyscalls, privacyExclude, nil, nil)

		patterns := []string{}
		if privacyRedactPatterns != "" {
			for _, s := range strings.Split(privacyRedactPatterns, ",") {
				s = strings.TrimSpace(s)
				if s != "" {
					patterns = append(patterns, s)
				}
			}
		}
		rcfg := predact.Config{NoArgs: derivedNoArgs, MaxArgSize: privacyMaxArgSize, Patterns: patterns}
		r, err := predact.New(rcfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to initialize redactor: %v; disabling privacy logging\n", err)
			pEnabled = false
		} else {
			pRedactor = r
		}

		pFormatter = pformat.NewJSONFormatter()

		if pEnabled {
			if privacyLogPath == "stdout" {
				pOutput = pout.NewStdout()
			} else {
				of, err := pout.NewFile(privacyLogPath, ttl, ctx)
				if err != nil {
					fmt.Fprintf(os.Stderr, "warning: cannot open privacy log %s: %v; disabling privacy logging\n", privacyLogPath, err)
					pEnabled = false
				} else {
					pOutput = of
				}
			}
		}

		// Initialize audit logger if privacy logging enabled and output created.
		if pEnabled && pOutput != nil {
			if privacyLogPath == "stdout" {
				// no audit file for stdout
			} else {
				al, err := paudit.New(privacyLogPath + ".audit")
				if err != nil {
					fmt.Fprintf(os.Stderr, "warning: cannot create audit log: %v\n", err)
				} else {
					auditLogger = al
					// Log initial entry
					if err := auditLogger.Log(paudit.Entry{
						"action": "trace_start",
						"label":  label,
						"privacy_opts": map[string]any{
							"no_args":       privacyNoArgs,
							"max_arg_size":  privacyMaxArgSize,
							"syscalls":      privacySyscalls,
							"exclude":       privacyExclude,
							"privacy_level": privacyPrivacyLevel,
							"full":          privacyFull,
							"ttl":           privacyTTL,
						},
						"program": "",
						"args":    "",
					}); err != nil {
						fmt.Fprintf(os.Stderr, "warning: audit log write failed: %v\n", err)
					}
				}
			}
		}
	}
	wg.Go(func() {
		defer close(done)
		for event := range events {
			agg.Add(event)
			if pEnabled && pOutput != nil {
				te := p.NewTraceEventFromModel(event)
				if derivedAllowFull {
					te.Args = []p.Arg{{Name: "raw", Value: []byte(event.Args)}}
				}
				if err := ppipeline.Process(&te, pFilter, pRedactor, pFormatter, pOutput); err != nil {
					fmt.Fprintf(os.Stderr, "warning: privacy pipeline error: %v\n", err)
				} else {
					if err := pOutput.Write([]byte("\n")); err != nil {
						fmt.Fprintf(os.Stderr, "warning: privacy log separator write failed: %v\n", err)
					}
					eventCount++
				}
			}
		}
		agg.SetDone()
	})

	var runErr error
	if serveAddr != "" {
		fmt.Fprintf(os.Stderr, "serving on %s\n", serveAddr)
		srv := server.New(serveAddr, agg, wsToken)
		runErr = srv.Start(ctx)
	} else {
		// If stdin is not a terminal (e.g., running under `go test`),
		// don't start the interactive TUI which would take over the
		// terminal and block the test process. Instead, wait for the
		// events consumer to finish draining `done` and continue.
		if term.IsTerminal(syscall.Stdin) {
			runErr = ui.Run(agg, label, done, nil)
		} else {
			<-done
			runErr = nil
		}
	}

	cancelTracer()
	wg.Wait()

	if pEnabled && pOutput != nil {
		if err := pOutput.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to close privacy log: %v\n", err)
		}
		if auditLogger != nil && privacyLogPath != "stdout" {
			f, err := os.Open(privacyLogPath)
			if err == nil {
				hasher := sha256.New()
				if _, err := io.Copy(hasher, f); err == nil {
					if err := auditLogger.Log(paudit.Entry{
						"action":      "trace_end",
						"label":       label,
						"event_count": eventCount,
						"file_hash":   hex.EncodeToString(hasher.Sum(nil)),
						"duration":    time.Since(startTime).String(),
					}); err != nil {
						fmt.Fprintf(os.Stderr, "warning: audit log write failed: %v\n", err)
					}
				}
				if err := f.Close(); err != nil {
					fmt.Fprintf(os.Stderr, "warning: failed to close privacy log after hashing: %v\n", err)
				}
			}
		}
	}

	if runErr == nil && reportPath != "" {
		if err := writeHTMLReport(reportPath, agg, label, reportTopFiles); err != nil {
			return err
		}
	}

	return runErr
}
