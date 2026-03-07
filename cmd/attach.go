package cmd

import (
	"context"
	"fmt"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/fabianoflorentino/stracectl/internal/aggregator"
	"github.com/fabianoflorentino/stracectl/internal/tracer"
)

var attachServeAddr string

var attachCmd = &cobra.Command{
	Use:   "attach [--serve :8080] <pid>",
	Short: "Attach to a running process and trace it",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		pid, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid PID %q: must be a number", args[0])
		}
		if pid <= 0 {
			return fmt.Errorf("PID must be a positive integer, got %d", pid)
		}

		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		// tracerCtx is cancelled either by a signal (via ctx) or explicitly
		// by runTrace once the UI/server exits, killing the strace subprocess.
		tracerCtx, cancelTracer := context.WithCancel(ctx)
		defer cancelTracer()

		agg := aggregator.New()
		t := tracer.NewStraceTracer()

		events, err := t.Attach(tracerCtx, pid)
		if err != nil {
			return err
		}

		return runTrace(ctx, cancelTracer, events, agg, attachServeAddr, fmt.Sprintf("PID %d", pid))
	},
}

// init initializes the attach command and adds it to the root command.
func init() {
	attachCmd.Flags().StringVar(&attachServeAddr, "serve", "",
		`expose HTTP API instead of TUI (e.g. --serve :8080)`)
	rootCmd.AddCommand(attachCmd)
}
