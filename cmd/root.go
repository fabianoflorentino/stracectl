package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

const (
	red   = "\033[31m"
	bold  = "\033[1m"
	reset = "\033[0m"
)

var rootCmd = &cobra.Command{
	Use:   "stracectl",
	Short: "A modern strace with real-time TUI",
	Long: `stracectl is a modern strace replacement with real-time aggregation,
per-syscall latency stats, and an interactive htop-style TUI.

Trace a command from the start, attach to a running process, or analyse a
saved strace log file offline. In any mode, pass --serve :8080 to replace
the TUI with an HTTP API, or --report report.html to write a self-contained
HTML report on exit.

Examples:
  stracectl run curl https://example.com           # trace a command from the start
  stracectl run --report out.html curl google.com  # trace and save an HTML report
  stracectl attach 1234                            # attach to a running process
  stracectl attach --serve :8080 1234              # attach and expose HTTP/Prometheus
  stracectl stats trace.log                        # analyse a saved strace file
  stracectl stats --serve :8080 trace.log          # serve stats from a saved file
  stracectl stats --report report.html trace.log   # analyse and export an HTML report
  stracectl discover myapp                         # find container PID in a Pod`,
}

func Execute() {
	rootCmd.SetErrPrefix(red + bold + "Error:" + reset)
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
