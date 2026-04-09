package output

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewStdoutWrite(t *testing.T) {
	// capture stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = old }()

	out := NewStdout()
	msg := []byte("hello\n")
	if err := out.Write(msg); err != nil {
		t.Fatalf("write stdout: %v", err)
	}
	// flush
	_ = w.Close()
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read pipe: %v", err)
	}
	if string(b) != string(msg) {
		t.Fatalf("expected %q got %q", string(msg), string(b))
	}
}

func TestNewFileAndWriteClose(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "out.log")
	out, err := NewFile(p, 0, context.Background())
	if err != nil {
		t.Fatalf("newfile: %v", err)
	}
	data := []byte("data")
	if err := out.Write(data); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(b) != string(data) {
		t.Fatalf("mismatch: %q", string(b))
	}
}

func TestNewFileRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte("x"), 0600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}
	if _, err := NewFile(link, 0, context.Background()); err == nil {
		t.Fatalf("expected error when creating output on symlink")
	}
}

func TestWriteCloseNil(t *testing.T) {
	o := &Output{w: nil}
	if err := o.Write([]byte("x")); err != nil {
		t.Fatalf("expected nil err when writing with nil writer, got %v", err)
	}
	if err := o.Close(); err != nil {
		t.Fatalf("expected nil err when closing nil writer, got %v", err)
	}
}

func TestNewFileAutoExpire(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "priv.log")
	out, err := NewFile(p, 100*time.Millisecond, context.Background())
	if err != nil {
		t.Fatalf("NewFile failed: %v", err)
	}
	if err := out.Write([]byte("hello\n")); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}

	// Poll until removed; do not rely on a single fixed sleep.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(p); os.IsNotExist(err) {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("expected file to be removed after TTL")
}

func TestNewFileAutoExpire_CancelledByContext(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "priv.log")

	ctx, cancel := context.WithCancel(context.Background())

	out, err := NewFile(p, 10*time.Second, ctx) // very long TTL
	if err != nil {
		t.Fatalf("NewFile: %v", err)
	}
	if err := out.Write([]byte("data\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	cancel() // cancel context; TTL goroutine must exit without removing the file

	// short grace period for goroutine to exit
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(p); os.IsNotExist(err) {
			t.Fatal("file was removed even though context was cancelled before TTL")
		}
		time.Sleep(5 * time.Millisecond)
	}
}
