package aggregator

import (
	"testing"
	"time"

	"github.com/fabianoflorentino/stracectl/internal/models"
)

func Test_CategoryBreakdown_Basic(t *testing.T) {
	a := New()
	// add a few events across syscalls
	a.Add(models.SyscallEvent{Name: "openat", Latency: 1 * time.Millisecond, Time: time.Now()})
	a.Add(models.SyscallEvent{Name: "openat", Latency: 2 * time.Millisecond, Time: time.Now(), Error: "ENOENT"})
	a.Add(models.SyscallEvent{Name: "read", Latency: 500 * time.Microsecond, Time: time.Now()})

	m := a.CategoryBreakdown()
	if len(m) == 0 {
		t.Fatalf("expected non-empty category breakdown map")
	}
	// total across categories should be at least total events
	var total int64
	for _, cs := range m {
		total += cs.Count
	}
	if total < a.Total() {
		t.Fatalf("category counts %d should be >= Total() %d", total, a.Total())
	}
}
