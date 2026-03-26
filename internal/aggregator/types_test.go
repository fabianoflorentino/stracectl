package aggregator

import (
	"testing"
	"time"
)

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

	if s.ErrPct() < 66.0 || s.ErrPct() > 67.0 {
		t.Errorf("ErrPct: want ~66.7, got %.2f", s.ErrPct())
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
