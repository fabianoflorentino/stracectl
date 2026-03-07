package aggregator_test

import (
	"testing"
	"time"

	"github.com/fabianoflorentino/stracectl/internal/aggregator"
	"github.com/fabianoflorentino/stracectl/internal/models"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func event(name string, latency time.Duration, errName string) models.SyscallEvent {
	return models.SyscallEvent{
		PID:     1,
		Name:    name,
		Latency: latency,
		Error:   errName,
		Time:    time.Now(),
	}
}

func ok(name string, latency time.Duration) models.SyscallEvent {
	return event(name, latency, "")
}

func fail(name string, latency time.Duration) models.SyscallEvent {
	return event(name, latency, "ENOENT")
}

// ── Add / basic counters ──────────────────────────────────────────────────────

func TestAdd_CountsAndTotals(t *testing.T) {
	a := aggregator.New()

	a.Add(ok("read", 10*time.Microsecond))
	a.Add(ok("read", 20*time.Microsecond))
	a.Add(fail("openat", 5*time.Microsecond))

	if got := a.Total(); got != 3 {
		t.Errorf("Total: want 3, got %d", got)
	}
	if got := a.Errors(); got != 1 {
		t.Errorf("Errors: want 1, got %d", got)
	}
	if got := a.UniqueCount(); got != 2 {
		t.Errorf("UniqueCount: want 2, got %d", got)
	}
}

// ── SyscallStat fields ────────────────────────────────────────────────────────

func TestStat_AvgMinMax(t *testing.T) {
	a := aggregator.New()
	a.Add(ok("write", 10*time.Microsecond))
	a.Add(ok("write", 30*time.Microsecond))
	a.Add(ok("write", 20*time.Microsecond))

	stats := a.Sorted(aggregator.SortByCount)
	if len(stats) != 1 {
		t.Fatalf("want 1 stat, got %d", len(stats))
	}
	s := stats[0]

	if s.Count != 3 {
		t.Errorf("Count: want 3, got %d", s.Count)
	}
	if want := 20 * time.Microsecond; s.AvgTime() != want {
		t.Errorf("AvgTime: want %v, got %v", want, s.AvgTime())
	}
	if want := 10 * time.Microsecond; s.MinTime != want {
		t.Errorf("MinTime: want %v, got %v", want, s.MinTime)
	}
	if want := 30 * time.Microsecond; s.MaxTime != want {
		t.Errorf("MaxTime: want %v, got %v", want, s.MaxTime)
	}
}

func TestStat_ErrPct(t *testing.T) {
	a := aggregator.New()
	a.Add(ok("access", 1*time.Microsecond))
	a.Add(fail("access", 1*time.Microsecond))
	a.Add(fail("access", 1*time.Microsecond))

	stats := a.Sorted(aggregator.SortByCount)
	s := stats[0]

	// 2 errors out of 3 calls = 66.6...%
	if s.ErrPct() < 66.0 || s.ErrPct() > 67.0 {
		t.Errorf("ErrPct: want ~66.7, got %.2f", s.ErrPct())
	}
}

// ── Category classification ───────────────────────────────────────────────────

func TestCategory_Classification(t *testing.T) {
	cases := []struct {
		syscall string
		wantCat aggregator.Category
		wantStr string
	}{
		{"read", aggregator.CatIO, "I/O"},
		{"openat", aggregator.CatIO, "I/O"},
		{"fstat", aggregator.CatFS, "FS"},
		{"lseek", aggregator.CatFS, "FS"},
		{"connect", aggregator.CatNet, "NET"},
		{"sendto", aggregator.CatNet, "NET"},
		{"mmap", aggregator.CatMem, "MEM"},
		{"mprotect", aggregator.CatMem, "MEM"},
		{"execve", aggregator.CatProcess, "PROC"},
		{"prctl", aggregator.CatProcess, "PROC"},
		{"rt_sigaction", aggregator.CatSignal, "SIG"},
		{"unknownsyscall", aggregator.CatOther, "OTHER"},
	}

	for _, tc := range cases {
		a := aggregator.New()
		a.Add(ok(tc.syscall, 1*time.Microsecond))
		stats := a.Sorted(aggregator.SortByCount)
		if len(stats) == 0 {
			t.Fatalf("%s: no stats returned", tc.syscall)
		}
		got := stats[0].Category
		if got != tc.wantCat {
			t.Errorf("%s: category want %v, got %v", tc.syscall, tc.wantCat, got)
		}
		if got.String() != tc.wantStr {
			t.Errorf("%s: String() want %q, got %q", tc.syscall, tc.wantStr, got.String())
		}
	}
}

// ── CategoryBreakdown ─────────────────────────────────────────────────────────

func TestCategoryBreakdown(t *testing.T) {
	a := aggregator.New()
	// 3 I/O events
	a.Add(ok("read", 1*time.Microsecond))
	a.Add(ok("read", 1*time.Microsecond))
	a.Add(ok("write", 1*time.Microsecond))
	// 2 NET events, 1 with error
	a.Add(ok("connect", 1*time.Microsecond))
	a.Add(fail("connect", 1*time.Microsecond))

	bd := a.CategoryBreakdown()

	io := bd[aggregator.CatIO]
	if io.Count != 3 {
		t.Errorf("CatIO count: want 3, got %d", io.Count)
	}
	if io.Errs != 0 {
		t.Errorf("CatIO errs: want 0, got %d", io.Errs)
	}

	net := bd[aggregator.CatNet]
	if net.Count != 2 {
		t.Errorf("CatNet count: want 2, got %d", net.Count)
	}
	if net.Errs != 1 {
		t.Errorf("CatNet errs: want 1, got %d", net.Errs)
	}
}

// ── SortByMax ─────────────────────────────────────────────────────────────────

func TestSorted_ByMax(t *testing.T) {
	a := aggregator.New()
	a.Add(ok("read", 5*time.Millisecond))   // max 5ms
	a.Add(ok("write", 50*time.Millisecond)) // max 50ms
	a.Add(ok("fstat", 1*time.Millisecond))  // max 1ms

	sorted := a.Sorted(aggregator.SortByMax)
	if sorted[0].Name != "write" {
		t.Errorf("SortByMax first: want write, got %s", sorted[0].Name)
	}
	if sorted[2].Name != "fstat" {
		t.Errorf("SortByMax last: want fstat, got %s", sorted[2].Name)
	}
}

// ── SortByErrors ─────────────────────────────────────────────────────────────

func TestSorted_ByErrors(t *testing.T) {
	a := aggregator.New()
	a.Add(fail("openat", 1*time.Microsecond))
	a.Add(fail("openat", 1*time.Microsecond))
	a.Add(fail("access", 1*time.Microsecond))

	sorted := a.Sorted(aggregator.SortByErrors)
	if sorted[0].Name != "openat" {
		t.Errorf("SortByErrors first: want openat, got %s", sorted[0].Name)
	}
}

// ── Rate ──────────────────────────────────────────────────────────────────────

func TestRate_InitiallyZero(t *testing.T) {
	a := aggregator.New()
	if r := a.Rate(); r != 0 {
		t.Errorf("Rate before any events: want 0, got %f", r)
	}
}

func TestRate_UpdatesAfterBurst(t *testing.T) {
	a := aggregator.New()

	// Add many events quickly, then sleep > 500ms to trigger the rate snapshot.
	for i := 0; i < 100; i++ {
		a.Add(ok("read", 1*time.Microsecond))
	}
	time.Sleep(600 * time.Millisecond)
	// One more event to trigger the rate recalculation.
	a.Add(ok("read", 1*time.Microsecond))

	if r := a.Rate(); r <= 0 {
		t.Errorf("Rate after burst: want > 0, got %f", r)
	}
}

// ── Concurrency safety ────────────────────────────────────────────────────────

func TestAdd_Concurrent(t *testing.T) {
	a := aggregator.New()
	done := make(chan struct{})

	const goroutines = 20
	const eventsEach = 500

	for i := 0; i < goroutines; i++ {
		go func() {
			for j := 0; j < eventsEach; j++ {
				a.Add(ok("read", time.Microsecond))
			}
			done <- struct{}{}
		}()
	}
	for i := 0; i < goroutines; i++ {
		<-done
	}

	want := int64(goroutines * eventsEach)
	if got := a.Total(); got != want {
		t.Errorf("concurrent Total: want %d, got %d", want, got)
	}
}
