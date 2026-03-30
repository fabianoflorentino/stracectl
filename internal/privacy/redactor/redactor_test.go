package redactor

import (
	"bytes"
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

func TestRedactor_RawPayloadAndNil(t *testing.T) {
	cfg := Config{MaxArgSize: 8}
	r, err := New(cfg)
	if err != nil {
		t.Fatalf("new redactor: %v", err)
	}

	// nil event should be handled
	if err := r.Redact(nil); err != nil {
		t.Fatalf("expected nil redact on nil event, got %v", err)
	}

	rp := []byte("email=foo@example.com token=sekret")
	e := &privacy.TraceEvent{RawPayload: rp}
	if err := r.Redact(e); err != nil {
		t.Fatalf("redact rawpayload: %v", err)
	}
	out := string(e.RawPayload)
	if contains(out, "foo@example.com") || contains(out, "sekret") {
		t.Fatalf("expected raw payload to be redacted/truncated, got: %s", out)
	}
}

func TestRedactor_NewInvalidPattern(t *testing.T) {
	if _, err := New(Config{Patterns: []string{"("}}); err == nil {
		t.Fatalf("expected error when compiling invalid regex pattern")
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

func TestMaskShortAndMultibyte(t *testing.T) {
	// test mask() directly: it should return a byte slice of '*' at least length 4
	r, err := New(Config{MaxArgSize: 0})
	if err != nil {
		t.Fatalf("new: %v", err)
	}

	// short input (<4 bytes) should produce at least 4 asterisks
	m := r.mask([]byte("ab"))
	if len(m) < 4 {
		t.Fatalf("expected mask length at least 4, got %d", len(m))
	}
	if !bytes.Equal(m, bytes.Repeat([]byte("*"), len(m))) {
		t.Fatalf("expected mask to be all '*', got %q", m)
	}

	// longer input preserves length
	long := []byte("verylongsecretdata")
	m2 := r.mask(long)
	if len(m2) != len(long) {
		t.Fatalf("expected mask length %d, got %d", len(long), len(m2))
	}
	if !bytes.Equal(m2, bytes.Repeat([]byte("*"), len(m2))) {
		t.Fatalf("expected mask to be all '*', got %q", m2)
	}

	// multibyte bytes: mask uses byte length, ensure no panic and returns '*' bytes
	mb := []byte("pä") // contains a multibyte rune
	m3 := r.mask(mb)
	if len(m3) < 4 {
		t.Fatalf("expected mask length at least 4 for multibyte input, got %d", len(m3))
	}
	if !bytes.Equal(m3, bytes.Repeat([]byte("*"), len(m3))) {
		t.Fatalf("expected mask to be all '*', got %q", m3)
	}
}
