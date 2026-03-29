package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var explainCmd = &cobra.Command{
	Use:   "explain",
	Short: "Explain what will be captured by the current privacy settings",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Privacy settings:\n")
		fmt.Printf("  privacy-log: %s\n", privacyLogPath)
		fmt.Printf("  privacy-level: %s\n", privacyPrivacyLevel)
		fmt.Printf("  no-args: %v\n", privacyNoArgs)
		fmt.Printf("  max-arg-size: %d\n", privacyMaxArgSize)
		fmt.Printf("  syscalls: %s\n", privacySyscalls)
		fmt.Printf("  exclude: %s\n", privacyExclude)
		fmt.Printf("  privacy-ttl: %s\n", privacyTTL)
		fmt.Printf("  redact-patterns: %s\n", privacyRedactPatterns)
		fmt.Printf("\nExample event (redacted):\n")
		fmt.Printf("  {\"ts\": ..., \"pid\": 123, \"syscall\": \"open\", \"args\": [{\"name\": \"path\", \"value\": \"/path/****\"}]}\n")
	},
}

func init() {
	rootCmd.AddCommand(explainCmd)
}
