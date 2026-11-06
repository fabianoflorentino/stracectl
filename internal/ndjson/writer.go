// Package ndjson writes syscall events as newline-delimited JSON.
package ndjson

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/fabianoflorentino/stracectl/internal/models"
)

// SchemaVersion identifies the event schema written by this package.
const SchemaVersion = 1

// Event is the stable NDJSON representation of a syscall event.
//
// Latency is expressed in nanoseconds. Time is encoded as RFC 3339 with
// nanosecond precision by encoding/json.
type Event struct {
	SchemaVersion int       `json:"schema_version"`
	Time          time.Time `json:"time"`
	PID           int       `json:"pid"`
	Name          string    `json:"name"`
	Args          string    `json:"args,omitempty"`
	RetVal        string    `json:"retval"`
	Error         string    `json:"error,omitempty"`
	Latency       int64     `json:"latency"`
}

// Writer encodes events into a newline-delimited JSON stream.
type Writer struct {
	enc    *json.Encoder
	closer io.Closer
	closed bool
}

// NewWriter creates a Writer that encodes to w. Closing the Writer closes w.
func NewWriter(w io.WriteCloser) *Writer {
	return &Writer{enc: json.NewEncoder(w), closer: w}
}

// NewFile creates a Writer backed by path. The file is created with mode 0600
// and truncated if it already exists. Symlink paths are rejected.
func NewFile(path string) (*Writer, error) {
	clean := filepath.Clean(path)
	if clean == "" || clean == "." {
		return nil, fmt.Errorf("invalid save-events path %q", path)
	}

	if fi, err := os.Lstat(clean); err == nil && fi.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("save-events path must not be a symlink: %s", path)
	}

	f, err := os.OpenFile(clean, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, fmt.Errorf("opening save-events file %s: %w", path, err)
	}

	return NewWriter(f), nil
}

// Write encodes one syscall event and appends a newline.
func (w *Writer) Write(e models.SyscallEvent) error {
	return w.enc.Encode(Event{
		SchemaVersion: SchemaVersion,
		Time:          e.Time,
		PID:           e.PID,
		Name:          e.Name,
		Args:          e.Args,
		RetVal:        e.RetVal,
		Error:         e.Error,
		Latency:       int64(e.Latency),
	})
}

// Close closes the underlying writer once.
func (w *Writer) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true
	if w.closer == nil {
		return nil
	}
	return w.closer.Close()
}
