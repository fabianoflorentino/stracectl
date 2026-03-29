package audit

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

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
