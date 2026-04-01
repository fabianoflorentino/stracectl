package render

import (
	"fmt"
	"strings"
	"testing"

	"github.com/fabianoflorentino/stracectl/internal/aggregator"
	"github.com/fabianoflorentino/stracectl/internal/procinfo"
	umodel "github.com/fabianoflorentino/stracectl/internal/ui/model"
)

// fakeAgg implements umodel.AggregatorView for category tests.
type fakeAgg struct {
	total     int64
	breakdown map[aggregator.Category]aggregator.CategoryStats
}

func (f fakeAgg) Total() int64                                           { return f.total }
func (f fakeAgg) Errors() int64                                          { return 0 }
func (f fakeAgg) Rate() float64                                          { return 0 }
func (f fakeAgg) UniqueCount() int                                       { return 0 }
func (f fakeAgg) Sorted(_ aggregator.SortField) []aggregator.SyscallStat { return nil }
func (f fakeAgg) RecentLog() []aggregator.LogEntry                       { return nil }
func (f fakeAgg) TopFiles(_ int) []aggregator.FileStat                   { return nil }
func (f fakeAgg) CategoryBreakdown() map[aggregator.Category]aggregator.CategoryStats {
	return f.breakdown
}
func (f fakeAgg) GetProcInfo() procinfo.ProcInfo { return procinfo.ProcInfo{} }

// ensure fakeAgg satisfies the interface at compile time.
var _ umodel.AggregatorView = fakeAgg{}

func TestRenderCategoryBar_Empty(t *testing.T) {
	agg := fakeAgg{
		total:     0,
		breakdown: map[aggregator.Category]aggregator.CategoryStats{},
	}
	out := RenderCategoryBar(agg, 80)
	// With no categories the bar should render without panicking and not contain any known category label.
	for _, label := range []string{"I/O", "FS", "NET", "MEM", "PROC", "SIG", "OTHER"} {
		if strings.Contains(out, label) {
			t.Errorf("empty bar should not contain %q, got: %q", label, out)
		}
	}
}

func TestRenderCategoryBar_SingleCategory(t *testing.T) {
	agg := fakeAgg{
		total: 100,
		breakdown: map[aggregator.Category]aggregator.CategoryStats{
			aggregator.CatIO: {Count: 100},
		},
	}
	out := RenderCategoryBar(agg, 80)
	if !strings.Contains(out, "I/O") {
		t.Errorf("expected I/O in output, got: %q", out)
	}
	if !strings.Contains(out, "100%") {
		t.Errorf("expected 100%% in output, got: %q", out)
	}
}

func TestRenderCategoryBar_MultipleCategories(t *testing.T) {
	agg := fakeAgg{
		total: 200,
		breakdown: map[aggregator.Category]aggregator.CategoryStats{
			aggregator.CatIO:  {Count: 100},
			aggregator.CatNet: {Count: 100},
		},
	}
	out := RenderCategoryBar(agg, 120)
	if !strings.Contains(out, "I/O") {
		t.Errorf("expected I/O in output, got: %q", out)
	}
	if !strings.Contains(out, "NET") {
		t.Errorf("expected NET in output, got: %q", out)
	}
	if !strings.Contains(out, "50%") {
		t.Errorf("expected 50%% (each category) in output, got: %q", out)
	}
}

func TestRenderCategoryBar_AllCategories(t *testing.T) {
	breakdown := map[aggregator.Category]aggregator.CategoryStats{
		aggregator.CatIO:      {Count: 10},
		aggregator.CatFS:      {Count: 10},
		aggregator.CatNet:     {Count: 10},
		aggregator.CatMem:     {Count: 10},
		aggregator.CatProcess: {Count: 10},
		aggregator.CatSignal:  {Count: 10},
		aggregator.CatOther:   {Count: 10},
	}
	agg := fakeAgg{total: 70, breakdown: breakdown}
	out := RenderCategoryBar(agg, 160)

	for _, label := range []string{"I/O", "FS", "NET", "MEM", "PROC", "SIG", "OTHER"} {
		if !strings.Contains(out, label) {
			t.Errorf("expected %q in output, got: %q", label, out)
		}
	}
}

func TestRenderCategoryBar_ZeroCountCategoryIsSkipped(t *testing.T) {
	agg := fakeAgg{
		total: 50,
		breakdown: map[aggregator.Category]aggregator.CategoryStats{
			aggregator.CatIO:  {Count: 50},
			aggregator.CatNet: {Count: 0},
		},
	}
	out := RenderCategoryBar(agg, 80)
	if !strings.Contains(out, "I/O") {
		t.Errorf("expected I/O in output, got: %q", out)
	}
	if strings.Contains(out, "NET") {
		t.Errorf("NET with 0 count should be skipped, got: %q", out)
	}
}

func TestCatStyle_NoPanic(t *testing.T) {
	categories := []aggregator.Category{
		aggregator.CatIO,
		aggregator.CatFS,
		aggregator.CatNet,
		aggregator.CatMem,
		aggregator.CatProcess,
		aggregator.CatSignal,
		aggregator.CatOther,
	}
	for _, cat := range categories {
		t.Run(cat.String(), func(t *testing.T) {
			s := catStyle(cat)
			// Ensure Render does not panic and returns non-empty output.
			out := s.Render(cat.String())
			if out == "" {
				t.Errorf("catStyle(%v).Render returned empty string", cat)
			}
		})
	}
}

func TestCatStyle_DifferentCategoriesHaveDifferentForegrounds(t *testing.T) {
	// Each category should map to a distinct foreground color.
	categories := []aggregator.Category{
		aggregator.CatIO,
		aggregator.CatFS,
		aggregator.CatNet,
		aggregator.CatMem,
		aggregator.CatProcess,
		aggregator.CatSignal,
		aggregator.CatOther,
	}
	seen := make(map[string]bool)
	for _, cat := range categories {
		fg := fmt.Sprintf("%v", catStyle(cat).GetForeground())
		seen[fg] = true
	}
	if len(seen) < 2 {
		t.Error("all categories share the same foreground color; expected at least two distinct colors")
	}
}
