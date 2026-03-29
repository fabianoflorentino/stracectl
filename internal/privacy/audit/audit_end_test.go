package audit

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestAudit_LogTraceEndWithFileHash(t *testing.T) {
	dir := t.TempDir()
	// create a privacy log file with known content
	plog := filepath.Join(dir, "privacy.log")
	content := []byte("line1\nline2\n")
	if err := os.WriteFile(plog, content, 0600); err != nil {
		t.Fatalf("write privacy log: %v", err)
	}

	// compute expected sha256
	h := sha256.Sum256(content)
	expect := hex.EncodeToString(h[:])

	// create audit logger and write trace_end entry
	auditPath := filepath.Join(dir, "privacy.log.audit")
	l, err := New(auditPath)
	if err != nil {
		t.Fatalf("create audit logger: %v", err)
	}
	defer l.Close()

	if err := l.Log(Entry{"action": "trace_end", "file_hash": expect, "event_count": 2}); err != nil {
		t.Fatalf("log trace_end: %v", err)
	}

	// read audit file and verify JSON contains trace_end and matching hash
	f, err := os.Open(auditPath)
	if err != nil {
		t.Fatalf("open audit file: %v", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		t.Fatalf("expected a line in audit file")
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(scanner.Text()), &parsed); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if parsed["action"] != "trace_end" {
		t.Fatalf("unexpected action: %v", parsed["action"])
	}
	if parsed["file_hash"] != expect {
		t.Fatalf("file_hash mismatch: got=%v want=%v", parsed["file_hash"], expect)
	}
}
