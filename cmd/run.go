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
var runBackend string
var runTryElevate bool
var runForceEbpf bool
var runUnfiltered bool

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

		// Apply eBPF-specific CLI options when supported by the selected tracer.
		applyEBPFOptions(tr, runForceEbpf, runUnfiltered)

		events, err := tr.Run(tracerCtx, args[0], args[1:])
		if err != nil {
			// If requested, try to re-exec with elevated memlock using sudo/prlimit
			if runTryElevate && os.Getenv("STRACECTL_TRIED_ELEVATE") != "1" {
				fmt.Fprintln(os.Stderr, "eBPF failed to load; attempting to re-run with elevated memlock via sudo/prlimit...")
				// Attempt to re-run the entire process with prlimit/sudo. If successful,
				// this call will not return because it will exit the current process.
				tryElevateAndRerun()
				// If tryElevateAndRerun returns, it failed.
				fmt.Fprintln(os.Stderr, "elevation attempt failed; continuing with original error")
			}
			return err
		}

		return runTrace(ctx, cancelTracer, events, agg, runServeAddr, wsToken, runReportPath, strings.Join(args, " "))
	},
}

func init() {
	runCmd.Flags().StringVar(&runServeAddr, "serve", "", `expose HTTP API instead of TUI (e.g. --serve :8080)`)
	runCmd.Flags().StringVar(&runReportPath, "report", "", "write a self-contained HTML report to this file on exit")
	runCmd.Flags().StringVar(&runBackend, "backend", "auto", "choose tracer backend: auto, ebpf, strace")
	runCmd.Flags().BoolVar(&runTryElevate, "try-elevate", false, "attempt to re-run the process with sudo/prlimit to raise RLIMIT_MEMLOCK when eBPF load fails")
	runCmd.Flags().BoolVar(&runForceEbpf, "force-ebpf", false, "fail when eBPF probe fails instead of falling back to strace")
	runCmd.Flags().BoolVar(&runUnfiltered, "unfiltered", false, "disable PGID filter and capture system-wide events (useful on WSL)")
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
