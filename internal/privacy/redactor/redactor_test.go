package redactor

import (
	"testing"

	"github.com/fabianoflorentino/stracectl/internal/privacy"
)

func TestRedactor_BasicPatterns(t *testing.T) {
	cfg := Config{MaxArgSize: 0}
	r, err := New(cfg)
	if err != nil {
		t.Fatalf("new redactor: %v", err)
	}

	e := &privacy.TraceEvent{}
	e.Args = []privacy.Arg{{Name: "hdr", Value: []byte("Authorization: Bearer abc.def.ghi email=foo@example.com token=sekret")}}

	if err := r.Redact(e); err != nil {
		t.Fatalf("redact: %v", err)
	}

	out := string(e.Args[0].Value)
	if contains(out, "abc.def.ghi") || contains(out, "foo@example.com") || contains(out, "sekret") {
		t.Fatalf("expected sensitive substrings to be redacted, got: %s", out)
	}
}

func TestRedactor_NoArgs(t *testing.T) {
	cfg := Config{NoArgs: true}
	r, err := New(cfg)
	if err != nil {
		t.Fatalf("new redactor: %v", err)
	}

	e := &privacy.TraceEvent{Args: []privacy.Arg{{Name: "a", Value: []byte("secret")}}}
	if err := r.Redact(e); err != nil {
		t.Fatalf("redact: %v", err)
	}
	if len(e.Args) != 0 {
		t.Fatalf("expected args cleared when NoArgs=true")
	}
}

func TestRedactor_Truncate(t *testing.T) {
	cfg := Config{MaxArgSize: 8}
	r, err := New(cfg)
	if err != nil {
		t.Fatalf("new redactor: %v", err)
	}

	long := []byte("verylongsecretdata")
	e := &privacy.TraceEvent{Args: []privacy.Arg{{Name: "a", Value: long}}}
	if err := r.Redact(e); err != nil {
		t.Fatalf("redact: %v", err)
	}
	if len(e.Args[0].Value) != 8 {
		t.Fatalf("expected truncated to 8 bytes, got %d", len(e.Args[0].Value))
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
