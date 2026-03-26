package aggregator

import (
	"testing"
	"time"

	"github.com/fabianoflorentino/stracectl/internal/models"
	"github.com/fabianoflorentino/stracectl/internal/procinfo"
)

// helpers used only in core tests
func event(name string, latency time.Duration, errName string) models.SyscallEvent {
	return models.SyscallEvent{PID: 1, Name: name, Latency: latency, Error: errName, Time: time.Now()}
}
func ok(name string, latency time.Duration) models.SyscallEvent { return event(name, latency, "") }
func fail(name string, latency time.Duration) models.SyscallEvent {
	return event(name, latency, "ENOENT")
}

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

func TestRate_InitiallyZero(t *testing.T) {
	a := New()
	if r := a.Rate(); r != 0 {
		t.Errorf("Rate before any events: want 0, got %f", r)
	}
}

func TestRate_UpdatesAfterBurst(t *testing.T) {
	a := New()

	for i := 0; i < 100; i++ {
		a.Add(ok("read", 1*time.Microsecond))
	}
	time.Sleep(600 * time.Millisecond)
	a.Add(ok("read", 1*time.Microsecond))

	if r := a.Rate(); r <= 0 {
		t.Errorf("Rate after burst: want > 0, got %f", r)
	}
}

func TestAdd_Concurrent(t *testing.T) {
	a := New()
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

func TestRecentLog_CappedAt500(t *testing.T) {
	a := New()
	for i := 0; i < 600; i++ {
		a.Add(ok("read", time.Microsecond))
	}
	log := a.RecentLog()
	if len(log) > 500 {
		t.Errorf("RecentLog should be capped at 500, got %d", len(log))
	}
}

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

// aggregated core methods
func TestAggregator_CoreMethods(t *testing.T) {
	a := New()
	if a.StartTime().IsZero() {
		t.Fatal("StartTime should be non-zero")
	}
	if a.Total() != 0 || a.Errors() != 0 || a.UniqueCount() != 0 {
		t.Fatalf("initial counters should be zero: total=%d errors=%d unique=%d", a.Total(), a.Errors(), a.UniqueCount())
	}
	a.Add(ok("read", 10*time.Microsecond))
	a.Add(fail("openat", 5*time.Microsecond))
	if a.Total() != 2 {
		t.Fatalf("Total after adds: want 2, got %d", a.Total())
	}
	if a.Errors() != 1 {
		t.Fatalf("Errors after adds: want 1, got %d", a.Errors())
	}
	p := procinfo.ProcInfo{PID: 1234, Comm: "foo", Exe: "/bin/foo"}
	a.SetProcInfo(p)
	got := a.GetProcInfo()
	if got != p {
		t.Fatalf("procinfo roundtrip: want %+v got %+v", p, got)
	}
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
	for i := 0; i < 200; i++ {
		a.Add(ok("read", 1*time.Microsecond))
	}
	time.Sleep(600 * time.Millisecond)
	a.Add(ok("read", 1*time.Microsecond))
	if a.Rate() <= 0 {
		t.Fatalf("Rate should be >0 after burst, got %f", a.Rate())
	}
	if a.IsDone() {
		t.Fatal("IsDone should be false initially")
	}
	a.SetDone()
	if !a.IsDone() {
		t.Fatal("IsDone should be true after SetDone")
	}
}
