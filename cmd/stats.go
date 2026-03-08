package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/fabianoflorentino/stracectl/internal/aggregator"
	"github.com/fabianoflorentino/stracectl/internal/parser"
	"github.com/fabianoflorentino/stracectl/internal/server"
	"github.com/fabianoflorentino/stracectl/internal/ui"
)

var statsServeAddr string
var statsReportPath string

var statsCmd = &cobra.Command{
	Use:   "stats <file>",
	Short: "Analyse a saved strace output file (post-mortem)",
	Long: `Parse a raw strace output file and display the same aggregated stats
as the live trace session — without needing the traced process.

The file must have been captured with strace -T (timing) for latency data.
All output modes available to the live commands are supported.

Examples:
  stracectl stats trace.log                       # analyse in TUI
  stracectl stats --serve :8080 trace.log         # serve via HTTP / WebSocket / Prometheus
  stracectl stats --report report.html trace.log  # analyse and export HTML report

Capture a trace file with strace:
  strace -T -o trace.log curl https://example.com`,
	Args: cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		path := args[0]
		f, err := os.Open(path) // #nosec G304 — path comes from a CLI argument supplied by the operator
		if err != nil {
			return fmt.Errorf("opening %s: %w", path, err)
		}
		defer func() { _ = f.Close() }()

		agg := aggregator.New()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			event, parseErr := parser.Parse(scanner.Text(), 0)
			if parseErr != nil {
				continue // skip malformed lines silently
			}
			if event != nil {
				agg.Add(*event)
			}
		}
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("reading %s: %w", path, err)
		}

		if agg.Total() == 0 {
			return fmt.Errorf("no syscall events found in %s — make sure the file was produced by strace", path)
		}

		if statsServeAddr != "" {
			fmt.Fprintf(os.Stderr, "serving on %s\n", statsServeAddr)
			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()
			srv := server.New(statsServeAddr, agg)
			return srv.Start(ctx)
		}

		if err := ui.Run(agg, path); err != nil {
			return err
		}

		if statsReportPath != "" {
			return writeHTMLReport(statsReportPath, agg, path)
		}
		return nil
	},
}

func init() {
	statsCmd.Flags().StringVar(&statsServeAddr, "serve", "", "expose HTTP API instead of TUI (e.g. --serve :8080)")
	statsCmd.Flags().StringVar(&statsReportPath, "report", "", "write a self-contained HTML report to this file after analysis")
	rootCmd.AddCommand(statsCmd)
}
