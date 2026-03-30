package pipeline

import (
	"bytes"
	"errors"
	"testing"
	"time"

	privacy "github.com/fabianoflorentino/stracectl/internal/privacy"
	"github.com/fabianoflorentino/stracectl/internal/privacy/filters"
	pf "github.com/fabianoflorentino/stracectl/internal/privacy/formatter"
	pr "github.com/fabianoflorentino/stracectl/internal/privacy/redactor"
)

type dummyFilter struct{ allow bool }

func (d dummyFilter) Allow(e *privacy.TraceEvent) bool { return d.allow }

type errRedactor struct{ err error }

func (r errRedactor) Redact(e *privacy.TraceEvent) error { return r.err }

type recordingFormatter struct{ called bool }

func (f *recordingFormatter) Format(e *privacy.TraceEvent) ([]byte, error) {
	f.called = true
	return []byte("ok"), nil
}

type recordingOutput struct{ written []byte }

func (o *recordingOutput) Write(b []byte) error { o.written = append(o.written, b...); return nil }
func (o *recordingOutput) Close() error         { return nil }

func TestProcess_FilterRejects(t *testing.T) {
	e := &privacy.TraceEvent{PID: 1}
	f := dummyFilter{allow: false}
	rf := &recordingFormatter{}
	ro := &recordingOutput{}
	if err := Process(e, f, nil, rf, ro); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if rf.called {
		t.Fatalf("formatter should not be called when filter rejects")
	}
}

func TestProcess_RedactorError(t *testing.T) {
	e := &privacy.TraceEvent{PID: 1}
	f := dummyFilter{allow: true}
	rr := errRedactor{err: errors.New("bad")}
	if err := Process(e, f, rr, nil, nil); err == nil {
		t.Fatalf("expected error from redactor to be propagated")
	}
}

func TestProcess_FormatterOutputNil(t *testing.T) {
	e := &privacy.TraceEvent{PID: 1}
	f := dummyFilter{allow: true}
	if err := Process(e, f, nil, nil, nil); err != nil {
		t.Fatalf("expected nil when formatter/output nil, got %v", err)
	}
}

func TestProcess_HappyPath(t *testing.T) {
	e := &privacy.TraceEvent{PID: 1}
	f := dummyFilter{allow: true}
	rf := &recordingFormatter{}
	ro := &recordingOutput{}
	if err := Process(e, f, nil, rf, ro); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !rf.called {
		t.Fatalf("formatter should have been called")
	}
	if string(ro.written) != "ok" {
		t.Fatalf("unexpected output written: %q", string(ro.written))
	}
}

// --- end of basic tests; additional end-to-end test follows ---

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
	if bytes.Contains([]byte(got), []byte("foo@example.com")) || bytes.Contains([]byte(got), []byte("sekret")) {
		t.Fatalf("sensitive data leaked in output: %s", got)
	}
}
