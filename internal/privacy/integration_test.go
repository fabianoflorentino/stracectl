package privacy_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	p "github.com/fabianoflorentino/stracectl/internal/privacy"
	paudit "github.com/fabianoflorentino/stracectl/internal/privacy/audit"
	redact "github.com/fabianoflorentino/stracectl/internal/privacy/redactor"
)

func TestAuditFileCreated(t *testing.T) {
	dir := t.TempDir()
	tracePath := filepath.Join(dir, "trace.json")
	auditPath := tracePath + ".audit"

	logger, err := paudit.New(auditPath)
	if err != nil {
		t.Fatalf("paudit.New: %v", err)
	}
	defer logger.Close()

	if err := logger.Log(paudit.Entry{"action": "trace_start", "label": "test"}); err != nil {
		t.Fatalf("log start: %v", err)
	}
	if err := logger.Log(paudit.Entry{"action": "trace_end", "event_count": 1}); err != nil {
		t.Fatalf("log end: %v", err)
	}

	// ensure file exists and contains the entries
	b, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("reading audit file: %v", err)
	}
	s := string(b)
	if s == "" || !contains(s, "trace_start") || !contains(s, "trace_end") {
		t.Fatalf("audit file missing expected entries: %s", s)
	}
}

func TestRedactor_NoArgs_RemovesArgs(t *testing.T) {
	r, err := redact.New(redact.Config{NoArgs: true, MaxArgSize: 64, Patterns: nil})
	if err != nil {
		t.Fatalf("new redactor: %v", err)
	}

	e := &p.TraceEvent{
		PID:  1234,
		Args: []p.Arg{{Name: "path", Value: []byte("/secret/path")}},
	}

	if err := r.Redact(e); err != nil {
		t.Fatalf("redact: %v", err)
	}

	if e.Args != nil {
		t.Fatalf("expected Args to be nil when NoArgs=true, got: %#v", e.Args)
	}
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }
