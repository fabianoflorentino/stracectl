package aggregator

import (
	"sync"
	"testing"
	"time"
)

// TestErrorBreakdown_PopulatedOnError verifies that the ErrorBreakdown map is
// correctly populated when events with errors are added to the aggregator.
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

// TestErrorBreakdown_NilWhenNoErrors verifies that the ErrorBreakdown field is
// nil for syscalls that have no recorded errors.
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

// TestErrorBreakdown_ConcurrentSafe verifies that the ErrorBreakdown map is updated
// correctly and safely when multiple goroutines add events with errors for the same syscall concurrently.
func TestErrorBreakdown_ConcurrentSafe(t *testing.T) {
	a := New()

	const goroutines = 20
	const eventsEach = 200

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for range goroutines {
		go func() {
			defer wg.Done()

			for j := range eventsEach {
				if j%2 == 0 {
					a.Add(event("openat", time.Microsecond, "ENOENT"))
				} else {
					a.Add(event("openat", time.Microsecond, "EACCES"))
				}
			}
		}()
	}

	wg.Wait()

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
