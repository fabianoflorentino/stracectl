package aggregator

import (
	"sync"
	"testing"
	"time"

	"github.com/fabianoflorentino/stracectl/internal/models"
	"github.com/fabianoflorentino/stracectl/internal/procinfo"
)

// helpers used only in core tests
func event(name string, latency time.Duration, errName string) models.SyscallEvent {
	return models.SyscallEvent{PID: 1, Name: name, Latency: latency, Error: errName, Time: time.Now()}
}

// ok and fail are just convenient wrappers around event() to create successful or failed events without repeating the error string.
func ok(name string, latency time.Duration) models.SyscallEvent { return event(name, latency, "") }

// fail creates an event with a fixed error string "ENOENT" to represent a failed syscall.
func fail(name string, latency time.Duration) models.SyscallEvent {
	return event(name, latency, "ENOENT")
}

// TestAdd_CountsAndTotals verifies that adding events correctly updates the total count, error count, and unique syscall count.
func TestAdd_CountsAndTotals(t *testing.T) {
	a := New()

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

// TestRate_InitiallyZero verifies that the rate is initially zero.
func TestRate_InitiallyZero(t *testing.T) {
	a := New()
	if r := a.Rate(); r != 0 {
		t.Errorf("Rate before any events: want 0, got %f", r)
	}
}

// TestRate_UpdatesAfterBurst verifies that the rate updates correctly after a burst of events.
func TestRate_UpdatesAfterBurst(t *testing.T) {
	var fakeNow time.Time
	fakeNow = time.Now()
	mu := sync.Mutex{}
	clockFn := func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		return fakeNow
	}
	advanceClock := func(d time.Duration) {
		mu.Lock()
		defer mu.Unlock()
		fakeNow = fakeNow.Add(d)
	}

	a := newWithClock(clockFn)

	for range 100 {
		a.Add(ok("read", 1*time.Microsecond))
	}

	// Advance fake clock past the 500ms rate-update threshold.
	advanceClock(600 * time.Millisecond)
	a.Add(ok("read", 1*time.Microsecond))

	if r := a.Rate(); r <= 0 {
		t.Errorf("Rate after burst: want > 0, got %f", r)
	}
}

// TestAddConcurrent verifies that adding events from multiple goroutines does not
// cause race conditions or incorrect counts.
func TestAddConcurrent(t *testing.T) {
	a := New()

	const goroutines = 20
	const eventsEach = 500

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for range goroutines {
		go func() {
			defer wg.Done()

			for range eventsEach {
				a.Add(ok("read", time.Microsecond))
			}
		}()
	}

	wg.Wait()

	want := int64(goroutines * eventsEach)
	if got := a.Total(); got != want {
		t.Errorf("concurrent Total: want %d, got %d", want, got)
	}
}

// TestRecentLog_RecordsEvents verifies that RecentLog returns the most recent
// events in the correct order and with the correct details.
func TestRecentLog_RecordsEvents(t *testing.T) {
	a := New()
	a.Add(ok("read", time.Microsecond))
	a.Add(fail("open", time.Microsecond))

	log := a.RecentLog()
	if len(log) != 2 {
		t.Fatalf("want 2 log entries, got %d", len(log))
	}

	if log[0].Name != "read" {
		t.Errorf("first entry: want read, got %s", log[0].Name)
	}

	if log[1].Name != "open" || log[1].Error != "ENOENT" {
		t.Errorf("second entry: want open/ENOENT, got %s/%s", log[1].Name, log[1].Error)
	}
}

// TestRecentLog_CappedAt500 verifies that RecentLog does not return more than
// 500 entries, even if more events have been added.
func TestRecentLog_CappedAt500(t *testing.T) {
	a := New()
	for range 600 {
		a.Add(ok("read", time.Microsecond))
	}

	log := a.RecentLog()
	if len(log) > 500 {
		t.Errorf("RecentLog should be capped at 500, got %d", len(log))
	}
}

// TestSetDone_Behaviour verifies that SetDone correctly updates the done state
// and that IsDone reflects this state.
func TestSetDone_Behaviour(t *testing.T) {
	a := New()
	if a.IsDone() {
		t.Error("IsDone should be false for a new aggregator")
	}

	a.SetDone()
	if !a.IsDone() {
		t.Error("IsDone should be true after SetDone()")
	}

	a.SetDone()
	if !a.IsDone() {
		t.Error("IsDone should still be true after multiple SetDone() calls")
	}
}

// TestSorted_ByMaxAndErrors verifies that the Sorted method correctly sorts
// syscall stats by maximum latency and error count.
func TestSorted_ByMaxAndErrors(t *testing.T) {
	a := New()
	a.Add(ok("read", 5*time.Millisecond))
	a.Add(ok("write", 50*time.Millisecond))
	a.Add(ok("fstat", 1*time.Millisecond))

	sorted := a.Sorted(SortByMax)
	if sorted[0].Name != "write" {
		t.Errorf("SortByMax first: want write, got %s", sorted[0].Name)
	}

	if sorted[2].Name != "fstat" {
		t.Errorf("SortByMax last: want fstat, got %s", sorted[2].Name)
	}

	a = New()
	a.Add(fail("openat", 1*time.Microsecond))
	a.Add(fail("openat", 1*time.Microsecond))
	a.Add(fail("access", 1*time.Microsecond))

	sorted = a.Sorted(SortByErrors)
	if sorted[0].Name != "openat" {
		t.Errorf("SortByErrors first: want openat, got %s", sorted[0].Name)
	}
}

// The following tests cover core methods of the Aggregator that are not specific
// to any particular aspect like percentiles or error breakdowns.
// They verify the basic functionality of adding events, retrieving stats, and maintaining internal state correctly.

// TestAggregator_InitialState verifies that a new Aggregator starts with zero counts and a non-zero start time.
func TestAggregator_InitialState(t *testing.T) {
	a := New()
	if a.StartTime().IsZero() {
		t.Fatal("StartTime should be non-zero")
	}
	if a.Total() != 0 || a.Errors() != 0 || a.UniqueCount() != 0 {
		t.Fatalf("initial counters should be zero: total=%d errors=%d unique=%d", a.Total(), a.Errors(), a.UniqueCount())
	}
}

// TestAggregator_AddAndCounters verifies that adding events correctly updates the total count, error count, and unique syscall count.
func TestAggregator_AddAndCounters(t *testing.T) {
	a := New()
	a.Add(ok("read", 10*time.Microsecond))
	a.Add(fail("openat", 5*time.Microsecond))
	if a.Total() != 2 {
		t.Fatalf("Total after adds: want 2, got %d", a.Total())
	}
	if a.Errors() != 1 {
		t.Fatalf("Errors after adds: want 1, got %d", a.Errors())
	}
}

// TestAggregator_GetAndSorted verifies that Get can retrieve stats for a known
// syscall and that Sorted returns a non-empty list of stats.
func TestAggregator_ProcInfoAndRecentLog(t *testing.T) {
	a := New()
	p := procinfo.ProcInfo{PID: 1234, Comm: "foo", Exe: "/bin/foo"}
	a.SetProcInfo(p)
	got := a.GetProcInfo()

	if got != p {
		t.Fatalf("procinfo roundtrip: want %+v got %+v", p, got)
	}

	a.Add(ok("read", time.Microsecond))
	a.Add(fail("open", time.Microsecond))
	log := a.RecentLog()
	if len(log) < 2 {
		t.Fatalf("RecentLog should have at least 2 entries, got %d", len(log))
	}

	stats := a.Sorted(SortByCount)
	if len(stats) == 0 {
		t.Fatal("Sorted returned no stats")
	}

	if _, ok := a.Get(stats[0].Name); !ok {
		t.Fatalf("Get should find syscall %s", stats[0].Name)
	}
}

// TestAggregator_RateAfterBurst verifies that the Rate method returns a positive
// value after a burst of events, indicating that the rate calculation is working.
func TestAggregator_RateAfterBurst(t *testing.T) {
	var fakeNow time.Time
	fakeNow = time.Now()
	mu := sync.Mutex{}
	clockFn := func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		return fakeNow
	}
	advanceClock := func(d time.Duration) {
		mu.Lock()
		defer mu.Unlock()
		fakeNow = fakeNow.Add(d)
	}

	a := newWithClock(clockFn)

	for range 200 {
		a.Add(ok("read", 1*time.Microsecond))
	}

	advanceClock(600 * time.Millisecond)
	a.Add(ok("read", 1*time.Microsecond))
	if a.Rate() <= 0 {
		t.Fatalf("Rate should be >0 after burst, got %f", a.Rate())
	}
}

// TestAggregator_DoneState verifies that the IsDone and SetDone methods correctly
// reflect the completion state of the Aggregator.
func TestAggregator_DoneState(t *testing.T) {
	a := New()
	if a.IsDone() {
		t.Fatal("IsDone should be false initially")
	}

	a.SetDone()
	if !a.IsDone() {
		t.Fatal("IsDone should be true after SetDone")
	}
}
