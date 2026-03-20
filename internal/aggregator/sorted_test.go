package aggregator

import (
	"testing"
	"time"

	"github.com/fabianoflorentino/stracectl/internal/models"
)

func TestSorted_VariousSortsAndStats(t *testing.T) {
	a := New()
	now := time.Now()

	// Multiple events to exercise histograms, errors and file attribution
	a.Add(models.SyscallEvent{Name: "read", Latency: 1000 * time.Nanosecond, Time: now})
	a.Add(models.SyscallEvent{Name: "read", Latency: 2000 * time.Nanosecond, Time: now})
	a.Add(models.SyscallEvent{Name: "write", Latency: 500 * time.Nanosecond, Error: "EIO", Time: now})
	a.Add(models.SyscallEvent{Name: "open", Args: "\"/tmp/a\", O_RDONLY", RetVal: "3", PID: 1, Time: now})
	a.Add(models.SyscallEvent{Name: "write", Latency: 800 * time.Nanosecond, Error: "EIO", Time: now})

	sorts := []SortField{
		SortByCount, SortByTotal, SortByAvg, SortByMin,
		SortByMax, SortByErrors, SortByName, SortByCategory,
	}

	for _, s := range sorts {
		out := a.Sorted(s)
		if len(out) == 0 {
			t.Fatalf("expected non-empty sorted output for sort %v", s)
		}
	}

	// Validate Get and computed fields
	stat, ok := a.Get("write")
	if !ok {
		t.Fatalf("expected write syscall stat")
	}
	if stat.Count == 0 {
		t.Fatalf("expected write count > 0")
	}

	// meaningful assertions instead of no-op calls
	if at := stat.AvgTime(); at <= 0 {
		t.Fatalf("expected AvgTime > 0, got %v", at)
	}
	if ep := stat.ErrPct(); ep <= 0 {
		t.Fatalf("expected ErrPct > 0, got %v", ep)
	}
	if stat.P99 < stat.P95 {
		t.Fatalf("expected P99 >= P95: P95=%v P99=%v", stat.P95, stat.P99)
	}

	// TopErrors should reflect recorded error samples for "write"
	topErrs := stat.TopErrors(0)
	if len(topErrs) == 0 {
		t.Fatalf("expected TopErrors to be populated for write, got nil")
	}
	if topErrs[0].Count == 0 {
		t.Fatalf("expected top error count > 0, got %d", topErrs[0].Count)
	}
}
