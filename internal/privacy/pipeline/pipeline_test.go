package pipeline

import (
	"bytes"
	"testing"
	"time"

	"github.com/fabianoflorentino/stracectl/internal/privacy"
	"github.com/fabianoflorentino/stracectl/internal/privacy/filters"
	pf "github.com/fabianoflorentino/stracectl/internal/privacy/formatter"
	pr "github.com/fabianoflorentino/stracectl/internal/privacy/redactor"
)

// memoryOutput implements privacy.Output for tests.
type memoryOutput struct {
	buf *bytes.Buffer
}

func (m *memoryOutput) Write(b []byte) error {
	_, err := m.buf.Write(b)
	return err
}
func (m *memoryOutput) Close() error { return nil }

func TestPipeline_EndToEnd(t *testing.T) {
	// Setup components
	f := filters.New("", "", nil, nil)
	rcfg := pr.Config{MaxArgSize: 0}
	red, err := pr.New(rcfg)
	if err != nil {
		t.Fatalf("new redactor: %v", err)
	}
	formatter := pf.NewJSONFormatter()

	buf := &bytes.Buffer{}
	out := &memoryOutput{buf: buf}

	e := &privacy.TraceEvent{
		Ts:      time.Now(),
		PID:     42,
		Syscall: "write",
		Args:    []privacy.Arg{{Name: "data", Value: []byte("email=foo@example.com token=sekret")}},
	}

	if err := Process(e, f, red, formatter, out); err != nil {
		t.Fatalf("process: %v", err)
	}

	got := buf.String()
	if contains(got, "foo@example.com") || contains(got, "sekret") {
		t.Fatalf("sensitive data leaked in output: %s", got)
	}
}

func contains(s, sub string) bool { return bytes.Contains([]byte(s), []byte(sub)) }
