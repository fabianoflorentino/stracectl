package cmd

import (
	"bytes"
	"os"
	"testing"
)

func TestExplainCommand_Output(t *testing.T) {
	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Ensure flags have default values
	privacyLogPath = ""
	privacyNoArgs = true
	privacyMaxArgSize = 64
	privacyRedactPatterns = ""
	privacySyscalls = ""
	privacyExclude = ""
	privacyPrivacyLevel = "high"

	explainCmd.Run(nil, nil)

	_ = w.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	os.Stdout = old

	out := buf.String()
	if out == "" {
		t.Fatalf("expected explain output, got empty")
	}
	if !bytes.Contains(buf.Bytes(), []byte("Privacy settings:")) {
		t.Fatalf("unexpected explain output: %s", out)
	}
}
