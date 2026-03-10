package cmd

import (
	"context"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/fabianoflorentino/stracectl/internal/aggregator"
	"github.com/fabianoflorentino/stracectl/internal/tracer"
)

var runServeAddr string
var runReportPath string
var runWsToken string

var runCmd = &cobra.Command{
	Use:   "run [--serve :8080] [--report <path>] <command> [args...]",
	Short: "Run a command and trace it",
	Long: `Spawn a command under strace and display live syscall statistics in the TUI.

Press q or Ctrl+C to stop. On exit, an optional self-contained HTML report can
be written to a file for sharing or archiving.

When --serve is enabled, stracectl exposes a Web dashboard with live syscall
log search/filter, anomaly alerts, process metadata, process-exit notification,
per-errno breakdown, and P95/P99 + rolling error-rate metrics.

Examples:
  sudo stracectl run curl https://example.com
  sudo stracectl run -- python3 app.py --port 8080
  sudo stracectl run --serve :8080 curl https://example.com
	sudo stracectl run --serve :8080 --report trace.html curl https://example.com
  sudo stracectl run --report trace.html curl https://example.com`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		// tracerCtx is cancelled either by a signal (via ctx) or explicitly
		// by runTrace once the UI/server exits, killing the strace subprocess.
		tracerCtx, cancelTracer := context.WithCancel(ctx)
		defer cancelTracer()

		agg := aggregator.New()
		t := tracer.NewStraceTracer()

		events, err := t.Run(tracerCtx, args[0], args[1:])
		if err != nil {
			return err
		}

		return runTrace(ctx, cancelTracer, events, agg, runServeAddr, runWsToken, runReportPath, strings.Join(args, " "))
	},
}

func init() {
	runCmd.Flags().StringVar(&runServeAddr, "serve", "", `expose HTTP API instead of TUI (e.g. --serve :8080)`)
	runCmd.Flags().StringVar(&runReportPath, "report", "", "write a self-contained HTML report to this file on exit")
	runCmd.Flags().StringVar(&runWsToken, "ws-token", "", "require a Bearer token for WebSocket connections")
	rootCmd.AddCommand(runCmd)
}
