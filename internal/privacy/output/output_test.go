package output

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewFileAutoExpire(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "priv.log")
	out, err := NewFile(p, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("NewFile failed: %v", err)
	}
	if err := out.Write([]byte("hello\n")); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}

	// Wait longer than TTL and verify file removed.
	time.Sleep(250 * time.Millisecond)
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatalf("expected file to be removed after TTL, stat err=%v", err)
	}
}
