package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fabianoflorentino/stracectl/internal/models"
	"github.com/fabianoflorentino/stracectl/internal/ndjson"
)

func setPrivacyForSaveEvents(t *testing.T, level string, noArgs bool, maxArgSize int, patterns, syscalls, exclude string) {
	t.Helper()

	prevLevel := privacyPrivacyLevel
	prevNoArgs := privacyNoArgs
	prevMaxArgSize := privacyMaxArgSize
	prevPatterns := privacyRedactPatterns
	prevSyscalls := privacySyscalls
	prevExclude := privacyExclude

	t.Cleanup(func() {
		privacyPrivacyLevel = prevLevel
		privacyNoArgs = prevNoArgs
		privacyMaxArgSize = prevMaxArgSize
		privacyRedactPatterns = prevPatterns
		privacySyscalls = prevSyscalls
		privacyExclude = prevExclude
	})

	privacyPrivacyLevel = level
	privacyNoArgs = noArgs
	privacyMaxArgSize = maxArgSize
	privacyRedactPatterns = patterns
	privacySyscalls = syscalls
	privacyExclude = exclude
}

func readNDJSON(t *testing.T, path string) []ndjson.Event {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	lines := bytes.Split(bytes.TrimSpace(data), []byte("\n"))
	events := make([]ndjson.Event, 0, len(lines))
	for _, line := range lines {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var event ndjson.Event
		if err := json.Unmarshal(line, &event); err != nil {
			t.Fatalf("unmarshal event: %v", err)
		}
		events = append(events, event)
	}
	return events
}

func TestSaveEventsSinkWritesRedactedEvent(t *testing.T) {
	setPrivacyForSaveEvents(t, "medium", false, 0, "", "", "")

	path := filepath.Join(t.TempDir(), "events.ndjson")
	sink, err := newSaveEventsSink(path)
	if err != nil {
		t.Fatalf("newSaveEventsSink: %v", err)
	}

	event := models.SyscallEvent{
		PID:     42,
		Name:    "openat",
		Args:    `AT_FDCWD, "/tmp/foo", token=sekret email=foo@example.com`,
		RetVal:  "3",
		Latency: 15 * time.Microsecond,
		Time:    time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC),
	}
	if err := sink.Write(event); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := sink.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	events := readNDJSON(t, path)
	if len(events) != 1 {
		t.Fatalf("got %d events; want 1", len(events))
	}

	got := events[0]
	if got.SchemaVersion != ndjson.SchemaVersion {
		t.Errorf("SchemaVersion = %d; want %d", got.SchemaVersion, ndjson.SchemaVersion)
	}
	if got.Name != "openat" || got.PID != 42 || got.RetVal != "3" {
		t.Errorf("unexpected event: %+v", got)
	}
	if got.Latency != int64(15*time.Microsecond) {
		t.Errorf("Latency = %d; want %d", got.Latency, int64(15*time.Microsecond))
	}
	if strings.Contains(got.Args, "sekret") || strings.Contains(got.Args, "foo@example.com") {
		t.Errorf("args were not redacted: %q", got.Args)
	}
	if !strings.Contains(got.Args, "****") {
		t.Errorf("expected redaction mask in args, got %q", got.Args)
	}
}

func TestSaveEventsSinkHighPrivacySuppressesArgs(t *testing.T) {
	setPrivacyForSaveEvents(t, "high", false, 64, "", "", "")

	path := filepath.Join(t.TempDir(), "events.ndjson")
	sink, err := newSaveEventsSink(path)
	if err != nil {
		t.Fatalf("newSaveEventsSink: %v", err)
	}

	event := models.SyscallEvent{PID: 7, Name: "read", Args: `3, "secret"`, RetVal: "6"}
	if err := sink.Write(event); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := sink.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	events := readNDJSON(t, path)
	if len(events) != 1 {
		t.Fatalf("got %d events; want 1", len(events))
	}
	if events[0].Args != "" {
		t.Errorf("Args = %q; want empty under high privacy", events[0].Args)
	}
}

func TestSaveEventsSinkAppliesSyscallFilter(t *testing.T) {
	setPrivacyForSaveEvents(t, "medium", false, 0, "", "", "openat")

	path := filepath.Join(t.TempDir(), "events.ndjson")
	sink, err := newSaveEventsSink(path)
	if err != nil {
		t.Fatalf("newSaveEventsSink: %v", err)
	}

	if err := sink.Write(models.SyscallEvent{PID: 1, Name: "openat"}); err != nil {
		t.Fatalf("Write openat: %v", err)
	}
	if err := sink.Write(models.SyscallEvent{PID: 1, Name: "read"}); err != nil {
		t.Fatalf("Write read: %v", err)
	}
	if err := sink.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	events := readNDJSON(t, path)
	if len(events) != 1 {
		t.Fatalf("got %d events; want 1", len(events))
	}
	if events[0].Name != "read" {
		t.Errorf("Name = %q; want read", events[0].Name)
	}
}

func TestOpenSaveEventsSinkWithoutPath(t *testing.T) {
	prev := saveEventsPath
	t.Cleanup(func() { saveEventsPath = prev })

	saveEventsPath = ""
	sink, err := openSaveEventsSink()
	if err != nil {
		t.Fatalf("openSaveEventsSink: %v", err)
	}
	if sink != nil {
		t.Fatal("expected nil sink when --save-events is unset")
	}
}

func TestLoadAggFromFileWithSink(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trace.log")
	content := "read(3, \"hello\", 5) = 5 <0.000042>\n" +
		"write(1, \"hello\", 5) = 5 <0.000012>\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	var got []models.SyscallEvent
	agg, err := loadAggFromFileWithSink(path, false, func(event models.SyscallEvent) error {
		got = append(got, event)
		return nil
	})
	if err != nil {
		t.Fatalf("loadAggFromFileWithSink: %v", err)
	}
	if agg.Total() != 2 {
		t.Errorf("aggregator total = %d; want 2", agg.Total())
	}
	if len(got) != 2 {
		t.Fatalf("sink received %d events; want 2", len(got))
	}
	if got[0].Name != "read" || got[1].Name != "write" {
		t.Errorf("unexpected sink events: %+v", got)
	}
}

func TestLoadAggFromFileWithSinkPropagatesError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trace.log")
	if err := os.WriteFile(path, []byte("read(3, \"hi\", 2) = 2 <0.000005>\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	wantErr := errors.New("sink failed")
	if _, err := loadAggFromFileWithSink(path, false, func(models.SyscallEvent) error {
		return wantErr
	}); !errors.Is(err, wantErr) {
		t.Fatalf("error = %v; want %v", err, wantErr)
	}
}
