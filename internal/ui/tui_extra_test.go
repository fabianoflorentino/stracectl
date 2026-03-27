package ui

import (
	"testing"
	"time"

	"github.com/fabianoflorentino/stracectl/internal/aggregator"
)

func Test_ModelFromAggregator_DefaultsAndView(t *testing.T) {
	agg := aggregator.New()
	m := ModelFromAggregator(agg, "proc")
	if m.SortBy() != aggregator.SortByCount {
		t.Fatalf("expected default sort by count, got %v", m.SortBy())
	}
	// width==0 should render initialising message
	out := m.View()
	if out == "" || out != "Initialising stracectl…" {
		t.Fatalf("expected initialising view, got %q", out)
	}
}

func Test_Update_ProcessDeadAndTickFallback(t *testing.T) {
	agg := aggregator.New()
	m := ModelFromAggregator(agg, "proc")

	// processDeadMsg should mark processDone
	mi, _ := m.Update(processDeadMsg{})
	m2 := mi.(model)
	if !m2.ProcessDone() {
		t.Fatalf("expected ProcessDone after processDeadMsg")
	}

	// tickMsg repeated should eventually set a fallback width/height
	m3 := ModelFromAggregator(agg, "proc")
	for i := 0; i < 6; i++ {
		mi, _ = m3.Update(tickMsg(time.Now()))
		m3 = mi.(model)
	}
	if m3.Width() == 0 || m3.Height() == 0 {
		t.Fatalf("expected fallback size to be set after ticks, got w=%d h=%d", m3.Width(), m3.Height())
	}
}
