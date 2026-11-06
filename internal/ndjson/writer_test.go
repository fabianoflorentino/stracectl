package ndjson

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fabianoflorentino/stracectl/internal/models"
)

type bufferCloser struct {
	bytes.Buffer
	closeErr error
	closed   bool
}

func (b *bufferCloser) Close() error {
	b.closed = true
	return b.closeErr
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }
func (failingWriter) Close() error              { return nil }

func sampleEvent() models.SyscallEvent {
	return models.SyscallEvent{
		PID:     123,
		Name:    "openat",
		Args:    `AT_FDCWD, "/etc/hosts", O_RDONLY`,
		RetVal:  "3",
		Error:   "",
		Latency: 42 * time.Microsecond,
		Time:    time.Date(2026, 11, 4, 12, 30, 0, 123456789, time.UTC),
	}
}

func TestWriteEncodesSchema(t *testing.T) {
	buf := &bufferCloser{}
	w := NewWriter(buf)

	if err := w.Write(sampleEvent()); err != nil {
		t.Fatalf("Write: %v", err)
	}

	var got Event
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.SchemaVersion != SchemaVersion {
		t.Errorf("SchemaVersion = %d; want %d", got.SchemaVersion, SchemaVersion)
	}
	if got.PID != 123 || got.Name != "openat" {
		t.Errorf("unexpected identity fields: %+v", got)
	}
	if got.Args != sampleEvent().Args {
		t.Errorf("Args = %q; want %q", got.Args, sampleEvent().Args)
	}
	if got.RetVal != "3" {
		t.Errorf("RetVal = %q; want %q", got.RetVal, "3")
	}
	if got.Latency != int64(42*time.Microsecond) {
		t.Errorf("Latency = %d; want %d", got.Latency, int64(42*time.Microsecond))
	}
	if !got.Time.Equal(sampleEvent().Time) {
		t.Errorf("Time = %v; want %v", got.Time, sampleEvent().Time)
	}
}

func TestWriteUsesOneLinePerEvent(t *testing.T) {
	buf := &bufferCloser{}
	w := NewWriter(buf)

	first := sampleEvent()
	second := sampleEvent()
	second.PID = 456
	second.Name = "close"

	if err := w.Write(first); err != nil {
		t.Fatalf("Write first: %v", err)
	}
	if err := w.Write(second); err != nil {
		t.Fatalf("Write second: %v", err)
	}

	lines := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines; want 2", len(lines))
	}

	var got Event
	if err := json.Unmarshal([]byte(lines[1]), &got); err != nil {
		t.Fatalf("unmarshal second line: %v", err)
	}
	if got.PID != 456 || got.Name != "close" {
		t.Errorf("unexpected second event: %+v", got)
	}
}

func TestNewFileWritesAndCloses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.ndjson")
	w, err := NewFile(path)
	if err != nil {
		t.Fatalf("NewFile: %v", err)
	}

	if err := w.Write(sampleEvent()); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	var got Event
	if err := json.Unmarshal(bytes.TrimSpace(data), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Name != "openat" {
		t.Errorf("Name = %q; want openat", got.Name)
	}
}

func TestNewFileRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.ndjson")
	if err := os.WriteFile(target, []byte("keep"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	link := filepath.Join(dir, "link.ndjson")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink not available: %v", err)
	}

	if _, err := NewFile(link); err == nil {
		t.Fatal("NewFile accepted a symlink")
	}
}

func TestWriteReturnsEncoderError(t *testing.T) {
	w := NewWriter(failingWriter{})
	if err := w.Write(sampleEvent()); err == nil {
		t.Fatal("Write returned nil error for a failing writer")
	}
}

func TestCloseIsIdempotentAndPropagatesError(t *testing.T) {
	buf := &bufferCloser{closeErr: io.ErrClosedPipe}
	w := NewWriter(buf)

	if err := w.Close(); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("Close error = %v; want %v", err, io.ErrClosedPipe)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("second Close = %v; want nil", err)
	}
	if !buf.closed {
		t.Fatal("underlying closer was not closed")
	}
}
