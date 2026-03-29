package formatter

import (
	"encoding/json"
	"testing"

	"github.com/fabianoflorentino/stracectl/internal/privacy"
)

func TestJSONFormatter_Format(t *testing.T) {
	f := NewJSONFormatter()

	e := &privacy.TraceEvent{
		Ts:      privacy.TraceEvent{}.Ts,
		PID:     1234,
		Comm:    "app",
		Syscall: "open",
		Args:    []privacy.Arg{{Name: "path", Value: []byte("/etc/passwd")}},
		Ret:     "3",
	}

	b, err := f.Format(e)
	if err != nil {
		t.Fatalf("format: %v", err)
	}

	var out map[string]interface{}
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if out["pid"].(float64) != 1234 {
		t.Fatalf("unexpected pid")
	}
	if out["syscall"].(string) != "open" {
		t.Fatalf("unexpected syscall")
	}
}
