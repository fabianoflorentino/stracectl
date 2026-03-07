package cmd

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/fabianoflorentino/stracectl/internal/aggregator"
	"github.com/fabianoflorentino/stracectl/internal/tracer"
	"github.com/fabianoflorentino/stracectl/internal/ui"
)

var runCmd = &cobra.Command{
	Use:                "run <command> [args...]",
	Short:              "Run a command and trace it",
	Args:               cobra.MinimumNArgs(1),
	DisableFlagParsing: true,
	RunE: func(c *cobra.Command, args []string) error {
		agg := aggregator.New()
		t := tracer.NewStraceTracer()

		events, err := t.Run(args[0], args[1:])
		if err != nil {
			return err
		}

		go func() {
			for event := range events {
				agg.Add(event)
			}
		}()

		return ui.Run(agg, strings.Join(args, " "))
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
}
