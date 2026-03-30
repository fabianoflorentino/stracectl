package audit

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewRejectsDotDot(t *testing.T) {
	if _, err := New("../evil"); err == nil {
		t.Fatalf("expected error for path containing ..")
	}
}

func TestNewRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte("x"), 0600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}
	if _, err := New(link); err == nil {
		t.Fatalf("expected New to reject symlink path")
	}
}

func TestLogWritesEntry(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.log")
	l, err := New(p)
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}
	defer l.Close()
	if err := l.Log(Entry{"action": "test"}); err != nil {
		t.Fatalf("log: %v", err)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	s := string(b)
	if s == "" || (len(s) > 0 && s[0] != '{') {
		t.Fatalf("unexpected log content: %q", s)
	}
}

func TestAuditLogger_WriteAndClose(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/audit.log"

	l, err := New(path)
	if err != nil {
		t.Fatalf("New audit logger: %v", err)
	}
	defer l.Close()

	entry := Entry{"action": "test", "detail": "value"}
	if err := l.Log(entry); err != nil {
		t.Fatalf("Log: %v", err)
	}

	// Ensure file exists and contains a JSON line with ts and actor/uid (actor may be empty in test env).
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open audit file: %v", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		t.Fatalf("expected a line in audit file")
	}
	line := scanner.Text()

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(line), &parsed); err != nil {
		t.Fatalf("invalid json in audit file: %v", err)
	}

	if _, ok := parsed["ts"]; !ok {
		t.Fatalf("expected ts in audit entry")
	}
	if parsed["action"] != "test" {
		t.Fatalf("unexpected action value: %v", parsed["action"])
	}

	// ensure no additional lines
	if scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) != "" {
			t.Fatalf("expected only one line in audit file")
		}
	}
}

func TestLogMarshalError(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "bad.log")
	l, err := New(p)
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}
	defer l.Close()

	// non-marshallable value (channel) should cause json.Marshal to error
	ch := make(chan int)
	if err := l.Log(Entry{"bad": ch}); err == nil {
		t.Fatalf("expected json.Marshal error when logging non-marshallable value")
	}
}

func TestLogAndCloseOnNilLogger(t *testing.T) {
	var l *Logger
	if err := l.Log(Entry{"a": "b"}); err != nil {
		t.Fatalf("expected nil when logging with nil logger, got %v", err)
	}
	if err := l.Close(); err != nil {
		t.Fatalf("expected nil when closing nil logger, got %v", err)
	}
}

func TestNewRejectsDirectoryPath(t *testing.T) {
	dir := t.TempDir()
	// pass directory path itself
	if _, err := New(dir); err == nil {
		t.Fatalf("expected error when audit path is a directory")
	}
}

func TestNewOpenFilePermissionDenied(t *testing.T) {
	dir := t.TempDir()
	// make directory non-writable to cause OpenFile to fail
	if err := os.Chmod(dir, 0500); err != nil {
		t.Skipf("chmod unsupported: %v", err)
	}
	defer func() { _ = os.Chmod(dir, 0700) }()

	p := filepath.Join(dir, "cannot_create.log")
	if _, err := New(p); err == nil {
		t.Fatalf("expected New to fail creating file in non-writable dir")
	}
}
