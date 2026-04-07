package output

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Output is an implementation of writing bytes somewhere. This package provides
// simple file and stdout writers. The interface matches internal/privacy.Output.
type Output struct {
	w io.WriteCloser
}

// NewStdout creates an Output that writes to stdout.
func NewStdout() *Output {
	return &Output{w: os.Stdout}
}

// NewFile creates an Output that writes to the specified file path with mode 0600.
// If autoExpire > 0 the file will be removed after the duration.
func NewFile(path string, autoExpire time.Duration, ctx context.Context) (*Output, error) {
	clean := filepath.Clean(path)
	if strings.Contains(clean, "..") {
		return nil, errors.New("invalid output path")
	}

	// If file exists, reject symlinks.
	if fi, err := os.Lstat(clean); err == nil {
		if fi.Mode()&os.ModeSymlink != 0 {
			return nil, errors.New("output path must not be a symlink")
		}
	}

	f, err := os.OpenFile(clean, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return nil, err
	}

	out := &Output{w: f}

	if autoExpire > 0 {
		// Launch background goroutine to securely remove file after expiry; ignore errors.
		go func(p string, d time.Duration, ctx context.Context) {
			select {
			case <-time.After(d):
				// proceed with overwrite + removal
			case <-ctx.Done():
				// context cancelled (process exiting); do not remove file
				return
			}
			// Attempt to overwrite the file with zeros before removal to reduce
			// chance of data recovery on disk. This is best-effort and not
			// guaranteed to be secure on modern filesystems.
			if fi, err := os.Stat(p); err == nil {
				if sz := fi.Size(); sz > 0 {
					// Path was validated earlier (Clean + symlink check), so
					// this use of a variable path is intentional. Suppress the
					// gosec G304 warning for this best-effort overwrite.
					// #nosec G304
					if f2, err := os.OpenFile(p, os.O_WRONLY, 0); err == nil {
						buf := make([]byte, 4096)
						var written int64
						for written < sz {
							toWrite := int64(len(buf))
							if rem := sz - written; rem < toWrite {
								toWrite = rem
							}
							n, _ := f2.Write(buf[:toWrite])
							if n <= 0 {
								break
							}
							written += int64(n)
						}
						// attempt to sync; ignore errors
						_ = f2.Sync()
						_ = f2.Close()
					}
				}
			}
			_ = os.Remove(p)
		}(clean, autoExpire, ctx)
	}

	return out, nil
}

// Write writes bytes to the underlying writer.
func (o *Output) Write(b []byte) error {
	if o.w == nil {
		return nil
	}
	_, err := o.w.Write(b)
	return err
}

// Close closes the underlying writer when possible.
func (o *Output) Close() error {
	if o.w == nil {
		return nil
	}
	return o.w.Close()
}
