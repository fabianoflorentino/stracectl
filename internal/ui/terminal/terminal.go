package terminal

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"golang.org/x/term"
)

// DetectFallbackSize tries several strategies to determine a reasonable
// terminal width/height when no WindowSizeMsg is available.
func DetectFallbackSize() (int, int) {
	if fdu := os.Stdout.Fd(); fdu != 0 {
		if fd, ok := SafeIntFromUintptr(fdu); ok {
			if w, h, err := term.GetSize(fd); err == nil && w > 0 && h > 0 {
				return w, h
			}
		}
	}
	if s := os.Getenv("COLUMNS"); s != "" {
		if w, err := strconv.Atoi(s); err == nil && w > 0 {
			if l := os.Getenv("LINES"); l != "" {
				if h, err2 := strconv.Atoi(l); err2 == nil && h > 0 {
					return w, h
				}
			}
			return w, 24
		}
	}
	return 80, 24
}

func SafeIntFromUintptr(u uintptr) (int, bool) {
	maxInt := int(^uint(0) >> 1)
	if u <= uintptr(maxInt) {
		// #nosec G115: converting uintptr to int is guarded by a bounds check
		return int(u), true
	}
	return 0, false
}

func RecordFallbackEvent(w, h int) {
	f, _ := os.OpenFile(filepath.Join(os.TempDir(), fmt.Sprintf("stracectl_ui_fallback.%d.log", os.Getpid())), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if f == nil {
		return
	}
	defer func() { _ = f.Close() }()
	_, _ = fmt.Fprintf(f, "%s fallback width=%d height=%d\n", time.Now().Format(time.RFC3339Nano), w, h)
}

func RecordUIEvent(ev string, w, h int) {
	f, _ := os.OpenFile(filepath.Join(os.TempDir(), fmt.Sprintf("stracectl_ui_events.%d.log", os.Getpid())), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if f == nil {
		return
	}
	defer func() { _ = f.Close() }()
	_, _ = fmt.Fprintf(f, "%s ev=%s width=%d height=%d\n", time.Now().Format(time.RFC3339Nano), ev, w, h)
}
