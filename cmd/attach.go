package cmd

import (
	"context"
	"fmt"
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

var attachCmd = &cobra.Command{
	Use:   "attach [--serve :8080] [--report <path>] [--container <name> | <pid>]",
	Short: "Attach to a running process and trace it",
	Long: `Attach strace to an already-running process by PID and display live syscall
statistics in the TUI.

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
  sudo stracectl attach --container myapp`,
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
		t := tracer.NewStraceTracer()

		events, err := t.Attach(tracerCtx, pid)
		if err != nil {
			return err
		}

		return runTrace(ctx, cancelTracer, events, agg, attachServeAddr, attachReportPath, fmt.Sprintf("PID %d", pid))
	},
}

// init initializes the attach command and adds it to the root command.
func init() {
	attachCmd.Flags().StringVar(&attachServeAddr, "serve", "", `expose HTTP API instead of TUI (e.g. --serve :8080)`)
	attachCmd.Flags().StringVar(&attachReportPath, "report", "", "write a self-contained HTML report to this file on exit")
	attachCmd.Flags().StringVar(&attachContainer, "container", "", "auto-discover and attach to the lowest PID matching this container name")
	rootCmd.AddCommand(attachCmd)
}
