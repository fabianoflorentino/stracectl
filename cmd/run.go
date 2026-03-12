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
var runBackend string

var runCmd = &cobra.Command{
	Use:   "run [--serve :8080] [--report <path>] [--ws-token <token>] [--backend auto|ebpf|strace] <command> [args...]",
	Short: "Run a command and trace it",
	Long: `Run a command under a tracing backend and display live syscall
statistics in the TUI.

By default ` + "--backend auto" + ` is used: this selects the eBPF backend when
the running kernel supports the required features (Linux >= 5.8) and the
binary was compiled with eBPF support; otherwise it falls back to the classic
strace subprocess tracer. Use ` + "--backend ebpf" + ` or ` + "--backend strace" + `
to force a specific backend.

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
	sudo stracectl run --report trace.html curl https://example.com
	# Force eBPF backend (requires eBPF-enabled build and kernel support)
	sudo stracectl run --backend ebpf --report trace-ebpf.html curl https://example.com
	# Force classic strace subprocess tracer
	sudo stracectl run --backend strace curl https://example.com`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		// tracerCtx is cancelled either by a signal (via ctx) or explicitly
		// by runTrace once the UI/server exits, killing the strace subprocess.
		tracerCtx, cancelTracer := context.WithCancel(ctx)
		defer cancelTracer()

		agg := aggregator.New()

		tr, err := tracer.Select(runBackend)
		if err != nil {
			return err
		}

		events, err := tr.Run(tracerCtx, args[0], args[1:])
		if err != nil {
			return err
		}

		return runTrace(ctx, cancelTracer, events, agg, runServeAddr, wsToken, runReportPath, strings.Join(args, " "))
	},
}

func init() {
	runCmd.Flags().StringVar(&runServeAddr, "serve", "", `expose HTTP API instead of TUI (e.g. --serve :8080)`)
	runCmd.Flags().StringVar(&runReportPath, "report", "", "write a self-contained HTML report to this file on exit")
	runCmd.Flags().StringVar(&runBackend, "backend", "auto", "choose tracer backend: auto, ebpf, strace")
	rootCmd.AddCommand(runCmd)
}
