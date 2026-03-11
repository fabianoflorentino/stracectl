package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestWsTokenPersistentFlagRegistered(t *testing.T) {
	f := rootCmd.PersistentFlags().Lookup("ws-token")
	if f == nil {
		t.Fatal("expected persistent flag 'ws-token' to be registered")
	}
}

func TestRunHelpContainsWsToken(t *testing.T) {
	var buf bytes.Buffer
	prevOut := rootCmd.OutOrStdout()
	rootCmd.SetOut(&buf)
	defer rootCmd.SetOut(prevOut)
	defer rootCmd.SetArgs(nil)

	rootCmd.SetArgs([]string{"run", "--help"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute help: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "--ws-token") {
		t.Fatalf("expected --ws-token in help output; got:\n%s", out)
	}
}
