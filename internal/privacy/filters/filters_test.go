package filters

import (
	"testing"

	"github.com/fabianoflorentino/stracectl/internal/privacy"
)

func TestAllow_IncludeExcludePID(t *testing.T) {
	f := New("open,connect", "read,write", []int{1001}, []int{})

	e := &privacy.TraceEvent{PID: 1001, Syscall: "open"}
	if !f.Allow(e) {
		t.Fatalf("expected allow open for pid 1001")
	}

	e2 := &privacy.TraceEvent{PID: 1001, Syscall: "read"}
	if f.Allow(e2) {
		t.Fatalf("expected exclude read to be rejected")
	}

	e3 := &privacy.TraceEvent{PID: 2000, Syscall: "open"}
	if f.Allow(e3) {
		t.Fatalf("expected pid 2000 to be rejected")
	}
}

func TestAllow_NoInclude(t *testing.T) {
	f := New("", "unlink", nil, nil)

	e := &privacy.TraceEvent{PID: 1, Syscall: "open"}
	if !f.Allow(e) {
		t.Fatalf("expected open allowed when include list empty")
	}

	e2 := &privacy.TraceEvent{PID: 1, Syscall: "unlink"}
	if f.Allow(e2) {
		t.Fatalf("expected unlink excluded")
	}
}
