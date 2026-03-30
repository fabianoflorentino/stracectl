package privacy

import (
	"testing"
	"time"

	"github.com/fabianoflorentino/stracectl/internal/models"
)

func TestNewTraceEventFromModel(t *testing.T) {
	now := time.Now()
	m := models.SyscallEvent{PID: 42, Name: "open", RetVal: "0", Error: "", Time: now}
	te := NewTraceEventFromModel(m)
	if te.PID != 42 || te.Syscall != "open" || te.Ret != "0" {
		t.Fatalf("unexpected mapping: %#v", te)
	}
	if !te.Ts.Equal(now) {
		t.Fatalf("timestamp mismatch")
	}
}
