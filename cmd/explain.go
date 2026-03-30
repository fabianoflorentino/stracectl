package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	p "github.com/fabianoflorentino/stracectl/internal/privacy"
	predact "github.com/fabianoflorentino/stracectl/internal/privacy/redactor"
)

var explainCmd = &cobra.Command{
	Use:   "explain",
	Short: "Explain what will be captured by the current privacy settings",
	Run: func(cmd *cobra.Command, args []string) {
		// Print configured privacy settings
		fmt.Printf("Privacy settings:\n")
		fmt.Printf("  privacy-log: %s\n", privacyLogPath)
		fmt.Printf("  privacy-level: %s\n", privacyPrivacyLevel)
		fmt.Printf("  no-args: %v\n", privacyNoArgs)
		fmt.Printf("  max-arg-size: %d\n", privacyMaxArgSize)
		fmt.Printf("  syscalls: %s\n", privacySyscalls)
		fmt.Printf("  exclude: %s\n", privacyExclude)
		fmt.Printf("  privacy-ttl: %s\n", privacyTTL)
		fmt.Printf("  redact-patterns: %s\n", privacyRedactPatterns)
		// Show compiled redact patterns and a redacted example
		patterns := []string{}
		if privacyRedactPatterns != "" {
			for _, s := range strings.Split(privacyRedactPatterns, ",") {
				s = strings.TrimSpace(s)
				if s != "" {
					patterns = append(patterns, s)
				}
			}
		}
		fmt.Printf("\nCompiled redact patterns:\n")
		if len(patterns) == 0 {
			fmt.Printf("  (default patterns apply)\n")
		} else {
			for _, ptn := range patterns {
				fmt.Printf("  %s\n", ptn)
			}
		}

		// Create a sample event and apply redaction to demonstrate output
		rcfg := predact.Config{NoArgs: privacyNoArgs, MaxArgSize: privacyMaxArgSize, Patterns: patterns}
		r, err := predact.New(rcfg)
		fmt.Printf("\nExample event (redacted):\n")
		if err != nil {
			fmt.Printf("  (failed to initialize redactor: %v)\n", err)
			fmt.Printf("  {\"ts\": ..., \"pid\": 123, \"syscall\": \"open\", \"args\": [{\"name\": \"path\", \"value\": \"/path/****\"}]}\n")
			return
		}
		ev := &p.TraceEvent{
			Ts:      time.Now().UTC(),
			PID:     123,
			Syscall: "open",
			Args:    []p.Arg{{Name: "path", Value: []byte("/home/user/.ssh/id_rsa token=sekret@example.com email=foo@example.com")}},
		}
		_ = r.Redact(ev)
		// Print a concise example showing redacted arg value
		if len(ev.Args) > 0 {
			fmt.Printf("  {\"ts\": ..., \"pid\": %d, \"syscall\": \"%s\", \"args\": [{\"name\": \"%s\", \"value\": %q}]}\n", ev.PID, ev.Syscall, ev.Args[0].Name, string(ev.Args[0].Value))
		} else {
			fmt.Printf("  (args suppressed by current settings)\n")
		}
	},
}

func init() {
	rootCmd.AddCommand(explainCmd)
}
