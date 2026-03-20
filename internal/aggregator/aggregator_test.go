package aggregator

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/fabianoflorentino/stracectl/internal/models"
	"github.com/fabianoflorentino/stracectl/internal/procinfo"
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

// ── SyscallStat fields ────────────────────────────────────────────────────────

func TestStat_AvgMinMax(t *testing.T) {
	a := New()
	a.Add(ok("write", 10*time.Microsecond))
	a.Add(ok("write", 30*time.Microsecond))
	a.Add(ok("write", 20*time.Microsecond))

	stats := a.Sorted(SortByCount)
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
	a := New()
	a.Add(ok("access", 1*time.Microsecond))
	a.Add(fail("access", 1*time.Microsecond))
	a.Add(fail("access", 1*time.Microsecond))

	stats := a.Sorted(SortByCount)
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
		wantCat Category
		wantStr string
	}{
		{"read", CatIO, "I/O"},
		{"openat", CatIO, "I/O"},
		{"fstat", CatFS, "FS"},
		{"lseek", CatFS, "FS"},
		{"connect", CatNet, "NET"},
		{"sendto", CatNet, "NET"},
		{"mmap", CatMem, "MEM"},
		{"mprotect", CatMem, "MEM"},
		{"execve", CatProcess, "PROC"},
		{"prctl", CatProcess, "PROC"},
		{"rt_sigaction", CatSignal, "SIG"},
		{"unknownsyscall", CatOther, "OTHER"},
	}

	for _, tc := range cases {
		a := New()
		a.Add(ok(tc.syscall, 1*time.Microsecond))
		stats := a.Sorted(SortByCount)
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
	a := New()
	// 3 I/O events
	a.Add(ok("read", 1*time.Microsecond))
	a.Add(ok("read", 1*time.Microsecond))
	a.Add(ok("write", 1*time.Microsecond))
	// 2 NET events, 1 with error
	a.Add(ok("connect", 1*time.Microsecond))
	a.Add(fail("connect", 1*time.Microsecond))

	bd := a.CategoryBreakdown()

	io := bd[CatIO]
	if io.Count != 3 {
		t.Errorf("CatIO count: want 3, got %d", io.Count)
	}
	if io.Errs != 0 {
		t.Errorf("CatIO errs: want 0, got %d", io.Errs)
	}

	net := bd[CatNet]
	if net.Count != 2 {
		t.Errorf("CatNet count: want 2, got %d", net.Count)
	}
	if net.Errs != 1 {
		t.Errorf("CatNet errs: want 1, got %d", net.Errs)
	}
}

// ── SortByMax ─────────────────────────────────────────────────────────────────

func TestSorted_ByMax(t *testing.T) {
	a := New()
	a.Add(ok("read", 5*time.Millisecond))   // max 5ms
	a.Add(ok("write", 50*time.Millisecond)) // max 50ms
	a.Add(ok("fstat", 1*time.Millisecond))  // max 1ms

	sorted := a.Sorted(SortByMax)
	if sorted[0].Name != "write" {
		t.Errorf("SortByMax first: want write, got %s", sorted[0].Name)
	}
	if sorted[2].Name != "fstat" {
		t.Errorf("SortByMax last: want fstat, got %s", sorted[2].Name)
	}
}

// ── SortByErrors ─────────────────────────────────────────────────────────────

func TestSorted_ByErrors(t *testing.T) {
	a := New()
	a.Add(fail("openat", 1*time.Microsecond))
	a.Add(fail("openat", 1*time.Microsecond))
	a.Add(fail("access", 1*time.Microsecond))

	sorted := a.Sorted(SortByErrors)
	if sorted[0].Name != "openat" {
		t.Errorf("SortByErrors first: want openat, got %s", sorted[0].Name)
	}
}

// ── ErrorBreakdown ────────────────────────────────────────────────────────────

func TestErrorBreakdown_PopulatedOnError(t *testing.T) {
	a := New()
	a.Add(event("openat", 1*time.Microsecond, "ENOENT"))
	a.Add(event("openat", 1*time.Microsecond, "ENOENT"))
	a.Add(event("openat", 1*time.Microsecond, "EACCES"))
	a.Add(ok("openat", 1*time.Microsecond))

	s, got := a.Get("openat")
	if !got {
		t.Fatal("openat not found in aggregator")
	}
	if s.ErrorBreakdown == nil {
		t.Fatal("ErrorBreakdown should not be nil when errors were recorded")
	}
	if s.ErrorBreakdown["ENOENT"] != 2 {
		t.Errorf("ENOENT count: want 2, got %d", s.ErrorBreakdown["ENOENT"])
	}
	if s.ErrorBreakdown["EACCES"] != 1 {
		t.Errorf("EACCES count: want 1, got %d", s.ErrorBreakdown["EACCES"])
	}
}

func TestErrorBreakdown_NilWhenNoErrors(t *testing.T) {
	a := New()
	a.Add(ok("read", 1*time.Microsecond))

	s, found := a.Get("read")
	if !found {
		t.Fatal("read not found")
	}
	if s.ErrorBreakdown != nil {
		t.Errorf("ErrorBreakdown should be nil for error-free syscall, got %v", s.ErrorBreakdown)
	}
}

func TestTopErrors_SortedDescending(t *testing.T) {
	a := New()
	a.Add(event("openat", 1*time.Microsecond, "ENOENT"))
	a.Add(event("openat", 1*time.Microsecond, "ENOENT"))
	a.Add(event("openat", 1*time.Microsecond, "ENOENT"))
	a.Add(event("openat", 1*time.Microsecond, "EACCES"))
	a.Add(event("openat", 1*time.Microsecond, "EPERM"))

	s, found := a.Get("openat")
	if !found {
		t.Fatal("openat not found")
	}
	top := s.TopErrors(0)
	if len(top) != 3 {
		t.Fatalf("TopErrors: want 3 entries, got %d", len(top))
	}
	if top[0].Errno != "ENOENT" || top[0].Count != 3 {
		t.Errorf("TopErrors[0]: want ENOENT×3, got %s×%d", top[0].Errno, top[0].Count)
	}
}

func TestTopErrors_LimitN(t *testing.T) {
	a := New()
	for _, errno := range []string{"ENOENT", "EACCES", "EPERM", "EINVAL"} {
		a.Add(event("openat", 1*time.Microsecond, errno))
	}
	s, _ := a.Get("openat")
	top := s.TopErrors(2)
	if len(top) != 2 {
		t.Errorf("TopErrors(2): want 2 entries, got %d", len(top))
	}
}

func TestTopErrors_EmptyWhenNoErrors(t *testing.T) {
	a := New()
	a.Add(ok("read", 1*time.Microsecond))

	s, _ := a.Get("read")
	if got := s.TopErrors(0); len(got) != 0 {
		t.Errorf("TopErrors on no-error syscall: want empty, got %v", got)
	}
}

func TestErrorBreakdown_ConcurrentSafe(t *testing.T) {
	a := New()
	done := make(chan struct{})
	const goroutines = 10
	const eventsEach = 200

	for i := 0; i < goroutines; i++ {
		go func() {
			for j := 0; j < eventsEach; j++ {
				if j%2 == 0 {
					a.Add(event("openat", time.Microsecond, "ENOENT"))
				} else {
					a.Add(event("openat", time.Microsecond, "EACCES"))
				}
			}
			done <- struct{}{}
		}()
	}
	for i := 0; i < goroutines; i++ {
		<-done
	}

	s, _ := a.Get("openat")
	total := int64(0)
	for _, cnt := range s.ErrorBreakdown {
		total += cnt
	}
	want := int64(goroutines * eventsEach)
	if total != want {
		t.Errorf("ErrorBreakdown concurrent total: want %d, got %d", want, total)
	}
}

// ── RecentErrors (ring buffer) ────────────────────────────────────────────────

func TestRecentErrors_CappedAtMax(t *testing.T) {
	a := New()
	// Add 15 error events — ring buffer caps at 10
	for i := 0; i < 15; i++ {
		e := models.SyscallEvent{
			Name:    "openat",
			Args:    fmt.Sprintf("arg%d", i),
			Error:   "ENOENT",
			Latency: time.Microsecond,
			Time:    time.Now(),
		}
		a.Add(e)
	}
	s, _ := a.Get("openat")
	if len(s.RecentErrors) > 10 {
		t.Errorf("RecentErrors should be capped at 10, got %d", len(s.RecentErrors))
	}
}

func TestRecentErrors_ContainsLatestArgs(t *testing.T) {
	a := New()
	// Fill ring buffer past capacity so only the last 10 remain
	for i := 0; i < 15; i++ {
		e := models.SyscallEvent{
			Name:    "openat",
			Args:    fmt.Sprintf("path%d", i),
			Error:   "ENOENT",
			Latency: time.Microsecond,
			Time:    time.Now(),
		}
		a.Add(e)
	}
	s, _ := a.Get("openat")
	last := s.RecentErrors[len(s.RecentErrors)-1]
	if last.Args != "path14" {
		t.Errorf("RecentErrors last args: want path14, got %s", last.Args)
	}
}

func TestRecentErrors_EmptyWhenNoErrors(t *testing.T) {
	a := New()
	a.Add(ok("read", 1*time.Microsecond))
	s, _ := a.Get("read")
	if len(s.RecentErrors) != 0 {
		t.Errorf("RecentErrors should be empty for error-free syscall, got %v", s.RecentErrors)
	}
}

// ── Rate ──────────────────────────────────────────────────────────────────────

func TestRate_InitiallyZero(t *testing.T) {
	a := New()
	if r := a.Rate(); r != 0 {
		t.Errorf("Rate before any events: want 0, got %f", r)
	}
}

func TestRate_UpdatesAfterBurst(t *testing.T) {
	a := New()

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

// ── Latency percentiles (P95/P99) ─────────────────────────────────────────────

func TestPercentile_ZeroWhenNoEvents(t *testing.T) {
	a := New()
	a.Add(fail("open", 0)) // error, zero latency
	s, _ := a.Get("open")
	if s.P95 != 0 || s.P99 != 0 {
		t.Errorf("P95/P99 with no positive latency: want 0/0, got %v/%v", s.P95, s.P99)
	}
}

func TestPercentile_P95LessThanOrEqualP99(t *testing.T) {
	a := New()
	for i := 1; i <= 100; i++ {
		a.Add(ok("read", time.Duration(i)*time.Microsecond))
	}
	s, _ := a.Get("read")
	if s.P95 > s.P99 {
		t.Errorf("P95 (%v) must be <= P99 (%v)", s.P95, s.P99)
	}
	if s.P99 == 0 {
		t.Error("P99 should be non-zero after 100 events with positive latency")
	}
}

func TestPercentile_UniformDistribution(t *testing.T) {
	// 100 events: 1µs … 100µs. P99 should be in the bucket containing 99µs.
	a := New()
	for i := 1; i <= 100; i++ {
		a.Add(ok("write", time.Duration(i)*time.Microsecond))
	}
	s, _ := a.Get("write")
	// P99 bucket lower bound must be ≥ 32µs (bucket for 64µs range) and ≤ 128µs
	if s.P99 < 32*time.Microsecond || s.P99 > 128*time.Microsecond {
		t.Errorf("P99 out of expected range [32µs, 128µs]: got %v", s.P99)
	}
}

func TestPercentile_SortedPopulatesBoth(t *testing.T) {
	a := New()
	for i := 1; i <= 50; i++ {
		a.Add(ok("fstat", time.Duration(i)*time.Microsecond))
	}
	stats := a.Sorted(SortByCount)
	if len(stats) == 0 {
		t.Fatal("Sorted returned no stats")
	}
	s := stats[0]
	if s.P95 == 0 || s.P99 == 0 {
		t.Errorf("Sorted should populate P95/P99; got P95=%v P99=%v", s.P95, s.P99)
	}
}

// ── ProcInfo ──────────────────────────────────────────────────────────────────

func TestReadProcInfo_Self(t *testing.T) {
	pid := os.Getpid()
	info := procinfo.Read(pid)
	if info.PID != pid {
		t.Errorf("PID: want %d, got %d", pid, info.PID)
	}
	if info.Comm == "" {
		t.Error("Comm should be non-empty for the current process")
	}
	if info.Exe == "" {
		t.Error("Exe should be non-empty for the current process")
	}
}

func TestReadProcInfo_NonExistentPID(t *testing.T) {
	// A very large PID that cannot exist.
	info := procinfo.Read(999999999)
	if info.PID != 999999999 {
		t.Errorf("PID should be set even when process doesn't exist; got %d", info.PID)
	}
	// All string fields should be empty when /proc/<pid> doesn't exist.
	if info.Comm != "" || info.Exe != "" || info.Cwd != "" || info.Cmdline != "" {
		t.Errorf("string fields should be empty for non-existent PID; got %+v", info)
	}
}

func TestSetGetProcInfo(t *testing.T) {
	a := New()
	want := procinfo.ProcInfo{PID: 42, Comm: "testbin", Exe: "/usr/bin/testbin"}
	a.SetProcInfo(want)
	got := a.GetProcInfo()
	if got != want {
		t.Errorf("GetProcInfo: want %+v, got %+v", want, got)
	}
}

func TestGetProcInfo_DefaultEmpty(t *testing.T) {
	a := New()
	info := a.GetProcInfo()
	if info.PID != 0 || info.Comm != "" {
		t.Errorf("default ProcInfo should be zero value; got %+v", info)
	}
}

// ── ErrRate60s sliding window ─────────────────────────────────────────────────

func TestErrRate60s_ReflectsRecentErrors(t *testing.T) {
	a := New()
	now := time.Now()
	for i := 0; i < 5; i++ {
		a.Add(models.SyscallEvent{Name: "read", Latency: time.Microsecond, Error: "EIO", Time: now})
	}
	// A successful call does not count.
	a.Add(models.SyscallEvent{Name: "read", Latency: time.Microsecond, Time: now})
	s, _ := a.Get("read")
	if s.ErrRate60s != 5 {
		t.Errorf("ErrRate60s: want 5, got %d", s.ErrRate60s)
	}
}

func TestErrRate60s_ZeroWhenNoErrors(t *testing.T) {
	a := New()
	a.Add(ok("write", time.Microsecond))
	s, _ := a.Get("write")
	if s.ErrRate60s != 0 {
		t.Errorf("ErrRate60s should be 0 for error-free syscall, got %d", s.ErrRate60s)
	}
}

func TestErrRate60s_ExpiresOldBuckets(t *testing.T) {
	a := New()
	// Add an error 61 seconds in the past (outside the 60-second window).
	old := time.Now().Add(-61 * time.Second)
	a.Add(models.SyscallEvent{Name: "open", Latency: time.Microsecond, Error: "ENOENT", Time: old})
	// Add a recent error.
	a.Add(models.SyscallEvent{Name: "open", Latency: time.Microsecond, Error: "ENOENT", Time: time.Now()})
	s, _ := a.Get("open")
	if s.ErrRate60s != 1 {
		t.Errorf("ErrRate60s: want 1 (old error expired), got %d", s.ErrRate60s)
	}
}

func TestErrRate60s_SortedPopulates(t *testing.T) {
	a := New()
	now := time.Now()
	for i := 0; i < 3; i++ {
		a.Add(models.SyscallEvent{Name: "stat", Latency: time.Microsecond, Error: "ENOENT", Time: now})
	}
	stats := a.Sorted(SortByErrors)
	if len(stats) == 0 {
		t.Fatal("no stats returned")
	}
	if stats[0].ErrRate60s != 3 {
		t.Errorf("Sorted ErrRate60s: want 3, got %d", stats[0].ErrRate60s)
	}
}

// ── Live log ring buffer ──────────────────────────────────────────────────────

func TestRecentLog_EmptyInitially(t *testing.T) {
	a := New()
	if log := a.RecentLog(); len(log) != 0 {
		t.Errorf("RecentLog should be empty initially, got %d entries", len(log))
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

func TestRecentLog_ReturnsLatestWhenFull(t *testing.T) {
	a := New()
	// fill to capacity
	for i := 0; i < 500; i++ {
		a.Add(ok("read", time.Microsecond))
	}
	// one more with a unique syscall name
	a.Add(ok("write", time.Microsecond))
	log := a.RecentLog()
	if log[len(log)-1].Name != "write" {
		t.Errorf("last entry should be write (latest), got %s", log[len(log)-1].Name)
	}
}

func TestSetDone_FalseByDefault(t *testing.T) {
	a := New()
	if a.IsDone() {
		t.Error("IsDone should be false for a new aggregator")
	}
}

func TestSetDone_TrueAfterCall(t *testing.T) {
	a := New()
	a.SetDone()
	if !a.IsDone() {
		t.Error("IsDone should be true after SetDone()")
	}
}

func TestSetDone_Idempotent(t *testing.T) {
	a := New()
	a.SetDone()
	a.SetDone()
	if !a.IsDone() {
		t.Error("IsDone should still be true after multiple SetDone() calls")
	}
}

func TestTopFiles_Counts(t *testing.T) {
	a := New()

	// two opens for /etc/hosts
	a.Add(models.SyscallEvent{Name: "open", Args: "\"/etc/hosts\", O_RDONLY", Time: time.Now()})
	a.Add(models.SyscallEvent{Name: "open", Args: "\"/etc/hosts\", O_RDONLY", Time: time.Now()})
	// one openat for /etc/ld.so.cache
	a.Add(models.SyscallEvent{Name: "openat", Args: "AT_FDCWD, \"/etc/ld.so.cache\", O_RDONLY", Time: time.Now()})
	// open + openat for /tmp/foo (total 2)
	a.Add(models.SyscallEvent{Name: "open", Args: "\"/tmp/foo\", O_RDONLY", Time: time.Now()})
	a.Add(models.SyscallEvent{Name: "openat", Args: "AT_FDCWD, \"/tmp/foo\", O_RDONLY", Time: time.Now()})

	files := a.TopFiles(0)
	if len(files) < 3 {
		t.Fatalf("TopFiles: want >=3 entries, got %d", len(files))
	}

	// build map for easy assertions
	m := make(map[string]int64)
	for _, f := range files {
		m[f.Path] = f.Count
	}

	if m["/etc/hosts"] != 2 {
		t.Errorf("/etc/hosts count: want 2, got %d", m["/etc/hosts"])
	}
	if m["/tmp/foo"] != 2 {
		t.Errorf("/tmp/foo count: want 2, got %d", m["/tmp/foo"])
	}
	if m["/etc/ld.so.cache"] != 1 {
		t.Errorf("/etc/ld.so.cache count: want 1, got %d", m["/etc/ld.so.cache"])
	}
}

func TestCategoryJSON(t *testing.T) {
	b, err := json.Marshal(CatIO)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var c Category
	if err := json.Unmarshal(b, &c); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if c != CatIO {
		t.Fatalf("roundtrip failed: got %v", c)
	}
}

func TestParseRetInt(t *testing.T) {
	if v, ok := parseRetInt("123"); !ok || v != 123 {
		t.Fatalf("parseRetInt decimal failed")
	}
	if v, ok := parseRetInt("0x1f"); !ok || v != 31 {
		t.Fatalf("parseRetInt hex failed: %d", v)
	}
	if _, ok := parseRetInt(""); ok {
		t.Fatalf("parseRetInt empty should fail")
	}
}

func TestParseFirstIntArg(t *testing.T) {
	if v, ok := parseFirstIntArg("4, foo"); !ok || v != 4 {
		t.Fatalf("parseFirstIntArg failed")
	}
	if _, ok := parseFirstIntArg(""); ok {
		t.Fatalf("parseFirstIntArg empty should fail")
	}
}

func TestUnescapeAndExtractPath(t *testing.T) {
	p := extractPathFromArgs("open", "\"/tmp/foo bar\", O_RDONLY")
	if p != "/tmp/foo bar" {
		t.Fatalf("expected /tmp/foo bar, got %q", p)
	}
	p2 := extractPathFromArgs("openat", "AT_FDCWD, \"/etc/hosts\", 0")
	if p2 != "/etc/hosts" {
		t.Fatalf("expected /etc/hosts, got %q", p2)
	}
	if extractPathFromArgs("read", "\"/tmp/x\"") != "" {
		t.Fatalf("expected empty for read")
	}
}

func TestTopFilesAndAttribution(t *testing.T) {
	a := New()
	a.Add(models.SyscallEvent{Name: "open", Args: "\"/tmp/foo\", O_RDONLY", RetVal: "3", PID: 1, Time: time.Now()})
	a.Add(models.SyscallEvent{Name: "close", Args: "3", RetVal: "", PID: 1, Time: time.Now()})
	files := a.TopFilesForSyscall("open", 0)
	if len(files) == 0 || files[0].Path != "/tmp/foo" {
		t.Fatalf("TopFilesForSyscall did not contain expected /tmp/foo; got %v", files)
	}
	top := a.TopFiles(0)
	found := false
	for _, f := range top {
		if f.Path == "/tmp/foo" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("TopFiles missing /tmp/foo")
	}
}
