package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/fabianoflorentino/stracectl/internal/discover"
)

var discoverCmd = &cobra.Command{
	Use:   "discover <container-name>",
	Short: "Find the PID of a container in a shared-PID-namespace Pod",
	Long: `Scans /proc for processes whose cgroup path contains <container-name>.
Useful inside a Kubernetes sidecar with shareProcessNamespace: true.`,
	Args: cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		pid, err := discover.LowestPIDInContainer(args[0])
		if err != nil {
			return err
		}
		fmt.Printf("%d\n", pid)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(discoverCmd)
}
