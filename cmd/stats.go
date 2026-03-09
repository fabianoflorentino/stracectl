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
	Use:   "stats [--serve :8080] [--report <path>] <file>",
	Short: "Analyse a saved strace output file (post-mortem)",
	Long: `Parse a raw strace output file and display the same aggregated stats
as the live trace session — without needing the traced process.

This includes per-errno breakdown, rolling error-rate metrics, recent error
samples, and P95/P99 syscall latency percentiles.

The file must have been captured with strace -T (timing) for latency data.
All output modes available to the live commands are supported.

When --serve is enabled, stracectl exposes the same Web dashboard used in live
mode, including live table filtering and anomaly alerts over parsed data.

Examples:
  stracectl stats trace.log                       # analyse in TUI
  stracectl stats --serve :8080 trace.log         # serve via HTTP / WebSocket / Prometheus
  stracectl stats --report report.html trace.log  # analyse and export HTML report

Capture a trace file with strace:
  strace -T -o trace.log curl https://example.com`,
	Args: cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		agg, err := loadAggFromFile(args[0])
		if err != nil {
			return err
		}

		if statsServeAddr != "" {
			fmt.Fprintf(os.Stderr, "serving on %s\n", statsServeAddr)
			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()
			srv := server.New(statsServeAddr, agg)
			if err := srv.Start(ctx); err != nil {
				return err
			}
		} else {
			if err := ui.Run(agg, args[0], nil); err != nil {
				return err
			}
		}

		if statsReportPath != "" {
			return writeHTMLReport(statsReportPath, agg, args[0])
		}
		return nil
	},
}

// loadAggFromFile reads a strace output file and returns an Aggregator
// populated with all parsed events.
//
// The scanner buffer is set to 512 KiB — the same limit used by the live
// tracer — so that lines containing large read/write argument dumps are not
// silently truncated (the default bufio limit is only 64 KiB).
func loadAggFromFile(path string) (*aggregator.Aggregator, error) {
	f, err := os.Open(path) // #nosec G304 — path comes from a CLI argument supplied by the operator
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	agg := aggregator.New()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 512*1024), 512*1024) // match live-tracer buffer size

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
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	if agg.Total() == 0 {
		return nil, fmt.Errorf("no syscall events found in %s — make sure the file was produced by strace", path)
	}

	return agg, nil
}

func init() {
	statsCmd.Flags().StringVar(&statsServeAddr, "serve", "", "expose HTTP API instead of TUI (e.g. --serve :8080)")
	statsCmd.Flags().StringVar(&statsReportPath, "report", "", "write a self-contained HTML report to this file after analysis")
	rootCmd.AddCommand(statsCmd)
}
