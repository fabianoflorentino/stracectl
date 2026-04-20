package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/fabianoflorentino/stracectl/internal/aggregator"
	"github.com/fabianoflorentino/stracectl/internal/tracer"
)

var runServeAddr string
var runReportPath string
var runReportTopFiles int
var runBackend string
var runTryElevate bool
var runForceEbpf bool
var runUnfiltered bool
var runPerPID bool

// selectTracer allows tests to substitute tracer selection logic.
// Defaults to the real tracer.Select function.
var selectTracer = tracer.Select

var runCmd = &cobra.Command{
	Use:   "run [--serve :8080] [--report <path>] [--ws-token <token>] [--backend auto|ebpf|strace] [--per-pid] <command> [args...]",
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

Use --per-pid to group syscall rows by PID instead of aggregating all
processes into a single row per syscall.

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
	sudo stracectl run --backend strace curl https://example.com
	# Group rows by PID (useful when tracing workloads that spawn children)
	sudo stracectl run --per-pid -- bash -lc 'for i in 1 2; do (sleep 0.1) & done; wait'`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		// tracerCtx is cancelled either by a signal (via ctx) or explicitly
		// by runTrace once the UI/server exits, killing the strace subprocess.
		tracerCtx, cancelTracer := context.WithCancel(ctx)
		defer cancelTracer()

		agg := aggregator.New()
		if runPerPID {
			agg.SetPerPID(true)
		}

		tr, err := selectTracer(runBackend)
		if err != nil {
			return err
		}

		// Apply eBPF-specific CLI options when supported by the selected tracer.
		applyEBPFOptions(tr, runForceEbpf, runUnfiltered)

		// Defer starting the tracer to runTrace so the TUI can initialize first
		return runTrace(ctx, tracerCtx, cancelTracer, tr, args[0], args[1:], agg, runServeAddr, wsToken, runReportPath, runReportTopFiles, strings.Join(args, " "))
	},
}

func init() {
	runCmd.Flags().StringVar(&runServeAddr, "serve", "", `expose HTTP API instead of TUI (e.g. --serve :8080)`)
	runCmd.Flags().StringVar(&runReportPath, "report", "", "write a self-contained HTML report to this file on exit")
	runCmd.Flags().IntVar(&runReportTopFiles, "report-top-files", 50, "number of top files to include in HTML report")
	runCmd.Flags().StringVar(&runBackend, "backend", "auto", "choose tracer backend: auto, ebpf, strace")
	runCmd.Flags().BoolVar(&runTryElevate, "try-elevate", false, "attempt to re-run the process with sudo/prlimit to raise RLIMIT_MEMLOCK when eBPF load fails")
	runCmd.Flags().BoolVar(&runForceEbpf, "force-ebpf", false, "fail when eBPF probe fails instead of falling back to strace")
	runCmd.Flags().BoolVar(&runUnfiltered, "unfiltered", false, "disable PGID filter and capture system-wide events (useful on WSL)")
	runCmd.Flags().BoolVar(&runPerPID, "per-pid", false, "group syscall statistics by PID instead of aggregating across all processes")
	rootCmd.AddCommand(runCmd)
}

// tryElevateAndRerun attempts to re-execute the current binary with elevated
// memlock using `prlimit --memlock=unlimited`. If not running as root it will
// prefix the call with `sudo`. The environment variable STRACECTL_TRIED_ELEVATE
// is set to avoid recursion.
func tryElevateAndRerun() {
	if os.Getenv("STRACECTL_TRIED_ELEVATE") == "1" {
		return
	}

	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot determine executable for elevation:", err)
		return
	}

	// Build the command args: prlimit --memlock=unlimited -- <exe> <original-args...>
	prlimitArgs := append([]string{"prlimit", "--memlock=unlimited", "--", exe}, os.Args[1:]...)

	var cmd *exec.Cmd
	if os.Geteuid() != 0 {
		// Use sudo so the user can enter a password if needed.
		// #nosec G702: invoked with fixed program name and sanitized args
		args := append([]string{}, prlimitArgs...)
		cmd = exec.CommandContext(context.Background(), "sudo", args...) // #nosec G702
	} else {
		// #nosec G702: invoked with fixed program name and sanitized args
		cmd = exec.CommandContext(context.Background(), "prlimit", append([]string{"--memlock=unlimited", "--", exe}, os.Args[1:]...)...) // #nosec G702
	}

	cmd.Env = append(os.Environ(), "STRACECTL_TRIED_ELEVATE=1")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "elevation execution failed:", err)
		return
	}

	// If the elevated process returns successfully, exit the current process.
	os.Exit(0)
}
