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

// runServeAddr is the address to serve the HTTP API on. If empty, the TUI will be used instead.
var runServeAddr string

var runCmd = &cobra.Command{
	Use:   "run [--serve :8080] <command> [args...]",
	Short: "Run a command and trace it",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		agg := aggregator.New()
		t := tracer.NewStraceTracer()

		events, err := t.Run(ctx, args[0], args[1:])
		if err != nil {
			return err
		}

		return runTrace(ctx, events, agg, runServeAddr, strings.Join(args, " "))
	},
}

func init() {
	runCmd.Flags().StringVar(&runServeAddr, "serve", "",
		`expose HTTP API instead of TUI (e.g. --serve :8080)`)
	rootCmd.AddCommand(runCmd)
}
