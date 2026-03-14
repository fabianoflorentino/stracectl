package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/fabianoflorentino/stracectl/internal/aggregator"
	"github.com/fabianoflorentino/stracectl/internal/discover"
	"github.com/fabianoflorentino/stracectl/internal/tracer"
)

var attachServeAddr string
var attachReportPath string
var attachContainer string
var backend string
var attachTryElevate bool
var attachForceEbpf bool
var attachUnfiltered bool

var attachCmd = &cobra.Command{
	Use:   "attach [--serve :8080] [--report <path>] [--ws-token <token>] [--backend auto|ebpf|strace] [--container <name> | <pid>]",
	Short: "Attach to a running process and trace it",
	Long: `Attach a tracer to an already-running process by PID and display live
syscall statistics in the TUI.

The tracing backend may be selected with ` + "--backend" + `:

- ` + "auto" + ` (default): pick eBPF when available (Linux >= 5.8 and the
	binary was built with eBPF support), otherwise fall back to the strace
	subprocess tracer.
- ` + "ebpf" + `: use the eBPF backend (requires an eBPF-enabled build and
	kernel support).
- ` + "strace" + `: force the classic strace subprocess tracer.

Press q or Ctrl+C to stop. On exit, an optional self-contained HTML report can
be written to a file for sharing or archiving.

When --serve is enabled, stracectl exposes a Web dashboard with live syscall
log search/filter, anomaly alerts, process metadata, process-exit notification,
per-errno breakdown, and P95/P99 + rolling error-rate metrics.

Examples:
	sudo stracectl attach 1234
	sudo stracectl attach "$(pgrep nginx | head -1)"
	sudo stracectl attach --serve :8080 1234
	sudo stracectl attach --report nginx.html 1234
	sudo stracectl attach --container myapp
	sudo stracectl attach --backend ebpf 1234
	sudo stracectl attach --backend strace 1234`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		var pid int
		var err error
		if attachContainer != "" {
			pid, err = discover.LowestPIDInContainer(attachContainer)
			if err != nil {
				return err
			}
		} else {
			if len(args) == 0 {
				return fmt.Errorf("must provide either a PID or --container <name>")
			}
			pid, err = strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid PID %q: must be a number", args[0])
			}
			if pid <= 0 {
				return fmt.Errorf("PID must be a positive integer, got %d", pid)
			}
		}

		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		// tracerCtx is cancelled either by a signal (via ctx) or explicitly
		// by runTrace once the UI/server exits, killing the strace subprocess.
		tracerCtx, cancelTracer := context.WithCancel(ctx)
		defer cancelTracer()

		agg := aggregator.New()
		agg.SetProcInfo(aggregator.ReadProcInfo(pid))

		t, err := tracer.Select(backend)
		if err != nil {
			return err
		}

		// Apply eBPF-specific CLI options when supported by the selected tracer.
		applyEBPFOptions(t, attachForceEbpf, attachUnfiltered)

		events, err := t.Attach(tracerCtx, pid)
		if err != nil {
			// If requested, try to re-exec with elevated memlock using sudo/prlimit
			if attachTryElevate && os.Getenv("STRACECTL_TRIED_ELEVATE") != "1" {
				fmt.Fprintln(os.Stderr, "eBPF failed to load; attempting to re-run with elevated memlock via sudo/prlimit...")
				tryElevateAndRerun()
				fmt.Fprintln(os.Stderr, "elevation attempt failed; continuing with original error")
			}
			return err
		}

		return runTrace(ctx, cancelTracer, events, agg, attachServeAddr, wsToken, attachReportPath, fmt.Sprintf("PID %d", pid))
	},
}

// applyEBPFOptions attempts to configure an eBPF-capable tracer if the
// provided tracer implements the expected setters. This avoids direct field
// access so the code compiles with and without the `ebpf` build tag.
func applyEBPFOptions(t tracer.Tracer, force, unfiltered bool) {
	type cfg interface {
		SetForce(bool)
		SetUnfiltered(bool)
	}

	if c, ok := t.(cfg); ok {
		c.SetForce(force)
		c.SetUnfiltered(unfiltered)
	}
}

// init initializes the attach command and adds it to the root command.
func init() {
	attachCmd.Flags().StringVar(&attachServeAddr, "serve", "", `expose HTTP API instead of TUI (e.g. --serve :8080)`)
	attachCmd.Flags().StringVar(&attachReportPath, "report", "", "write a self-contained HTML report to this file on exit")
	attachCmd.Flags().StringVar(&attachContainer, "container", "", "auto-discover and attach to the lowest PID matching this container name")
	attachCmd.Flags().StringVar(&backend, "backend", "auto", "tracing backend to use: auto, ebpf, or strace")
	attachCmd.Flags().BoolVar(&attachTryElevate, "try-elevate", false, "attempt to re-run the process with sudo/prlimit to raise RLIMIT_MEMLOCK when eBPF load fails")
	attachCmd.Flags().BoolVar(&attachForceEbpf, "force-ebpf", false, "fail when eBPF probe fails instead of falling back to strace")
	attachCmd.Flags().BoolVar(&attachUnfiltered, "unfiltered", false, "disable PGID filter and capture system-wide events (useful on WSL)")
	rootCmd.AddCommand(attachCmd)
}
