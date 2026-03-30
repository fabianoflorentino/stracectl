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

func TestAllow_EmptySyscallWithInclude(t *testing.T) {
	f := New("open", "", nil, nil)
	e := &privacy.TraceEvent{PID: 1, Syscall: ""}
	if f.Allow(e) {
		t.Fatalf("expected empty syscall to be rejected when include list non-empty")
	}
}

func TestAllow_UIDFiltering(t *testing.T) {
	f := New("", "", nil, []int{200})
	e := &privacy.TraceEvent{UID: 100, Syscall: "open"}
	if f.Allow(e) {
		t.Fatalf("expected event with UID 100 to be rejected when UID filter only allows 200")
	}
	e2 := &privacy.TraceEvent{UID: 200, Syscall: "open"}
	if !f.Allow(e2) {
		t.Fatalf("expected event with UID 200 to be allowed")
	}
}
