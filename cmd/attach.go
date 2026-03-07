package cmd

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/fabianoflorentino/stracectl/internal/aggregator"
	"github.com/fabianoflorentino/stracectl/internal/tracer"
	"github.com/fabianoflorentino/stracectl/internal/ui"
)

var attachCmd = &cobra.Command{
	Use:   "attach <pid>",
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

		return ui.Run(agg, fmt.Sprintf("PID %d", pid))
	},
}

func init() {
	rootCmd.AddCommand(attachCmd)
}
