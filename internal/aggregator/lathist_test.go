package aggregator

import (
	"testing"
	"time"
)

func TestPercentile_UniformDistribution(t *testing.T) {
	a := New()
	for i := 1; i <= 100; i++ {
		a.Add(ok("write", time.Duration(i)*time.Microsecond))
	}
	s, _ := a.Get("write")
	if s.P99 < 32*time.Microsecond || s.P99 > 128*time.Microsecond {
		t.Errorf("P99 out of expected range [32µs, 128µs]: got %v", s.P99)
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
