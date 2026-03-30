package audit

import (
	"encoding/json"
	"errors"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"
)

// Logger writes simple audit entries as newline-delimited JSON.
type Logger struct {
	f *os.File
}

// New creates or appends to the given audit file path (mode 0600).
func New(path string) (*Logger, error) {
	clean := filepath.Clean(path)

	if strings.Contains(clean, "..") {
		return nil, errors.New("invalid audit path")
	}

	// If file exists, reject if it's a symlink.
	if fi, err := os.Lstat(clean); err == nil {
		if fi.Mode()&os.ModeSymlink != 0 {
			return nil, errors.New("audit path must not be a symlink")
		}
	}

	f, err := os.OpenFile(clean, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, err
	}

	// Ensure the created file is a regular file.
	if fi, err := f.Stat(); err == nil {
		if !fi.Mode().IsRegular() {
			if err := f.Close(); err != nil {
				return nil, err
			}

			return nil, errors.New("audit path is not a regular file")
		}
	}

	return &Logger{f: f}, nil
}

// Entry represents a single audit log entry.
type Entry map[string]any

// Log writes the entry with timestamp and actor info.
func (l *Logger) Log(e Entry) error {
	if l == nil || l.f == nil {
		return nil
	}

	// Add timestamp and actor
	e["ts"] = time.Now().UTC().Format(time.RFC3339)

	if u, err := user.Current(); err == nil {
		e["actor"] = u.Username
		e["uid"] = u.Uid
	}

	b, err := json.Marshal(e)
	if err != nil {
		return err
	}

	if _, err := l.f.Write(append(b, '\n')); err != nil {
		return err
	}

	return nil
}

// Close closes the underlying file.
func (l *Logger) Close() error {
	if l == nil || l.f == nil {
		return nil
	}

	return l.f.Close()
}
