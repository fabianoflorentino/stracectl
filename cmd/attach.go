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
	"github.com/fabianoflorentino/stracectl/internal/server"
	"github.com/fabianoflorentino/stracectl/internal/tracer"
	"github.com/fabianoflorentino/stracectl/internal/ui"
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

		agg := aggregator.New()
		t := tracer.NewStraceTracer()

		events, err := t.Attach(pid)
		if err != nil {
			return err
		}

		go func() {
			for event := range events {
				agg.Add(event)
			}
		}()

		if attachServeAddr != "" {
			fmt.Fprintf(os.Stderr, "serving on %s\n", attachServeAddr)
			srv := server.New(attachServeAddr, agg)
			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()
			return srv.Start(ctx)
		}

		return ui.Run(agg, fmt.Sprintf("PID %d", pid))
	},
}

func init() {
	attachCmd.Flags().StringVar(&attachServeAddr, "serve", "",
		`expose HTTP API instead of TUI (e.g. --serve :8080)`)
	rootCmd.AddCommand(attachCmd)
}
