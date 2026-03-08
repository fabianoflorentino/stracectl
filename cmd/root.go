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

Examples:
  stracectl attach 1234        # attach to running process
  stracectl run curl google.com  # trace a command`,
}

func Execute() {
	rootCmd.SetErrPrefix(red + bold + "Error:" + reset)
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
