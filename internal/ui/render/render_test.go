package render

import (
	"strings"
	"testing"
	"time"

	"github.com/fabianoflorentino/stracectl/internal/aggregator"
	"github.com/fabianoflorentino/stracectl/internal/models"
	umodel "github.com/fabianoflorentino/stracectl/internal/ui/model"
)

type fakeCtrl struct {
	w, h        int
	agg         *aggregator.Aggregator
	sort        aggregator.SortField
	filter      string
	editing     bool
	done        bool
	cursor      int
	logOffset   int
	filesOffset int
	started     time.Time
	target      string
}

func (f fakeCtrl) Width() int                   { return f.w }
func (f fakeCtrl) Height() int                  { return f.h }
func (f fakeCtrl) Agg() umodel.AggregatorView   { return f.agg }
func (f fakeCtrl) SortBy() aggregator.SortField { return f.sort }
func (f fakeCtrl) Filter() string               { return f.filter }
func (f fakeCtrl) Editing() bool                { return f.editing }
func (f fakeCtrl) ProcessDone() bool            { return f.done }
func (f fakeCtrl) Cursor() int                  { return f.cursor }
func (f fakeCtrl) LogOffsetPtr() *int           { return &f.logOffset }
func (f fakeCtrl) FilesOffsetPtr() *int         { return &f.filesOffset }
func (f fakeCtrl) Started() time.Time           { return f.started }
func (f fakeCtrl) Target() string               { return f.target }
func (f fakeCtrl) PerPID() bool                 { return false }

func Test_RenderDetailContainsSections(t *testing.T) {
	agg := aggregator.New()
	agg.Add(models.SyscallEvent{Name: "read", Latency: 10 * time.Microsecond, Time: time.Now()})
	out := RenderDetail(agg, aggregator.SortByCount, "", 0, 80, 24)
	if !strings.Contains(out, "SYSCALL REFERENCE") {
		t.Fatalf("RenderDetail missing 'SYSCALL REFERENCE'\nOutput: %s", out)
	}
	if !strings.Contains(out, "LIVE STATISTICS") {
		t.Fatalf("RenderDetail missing 'LIVE STATISTICS'\nOutput: %s", out)
	}
}

func Test_RenderViewProducesMainTable(t *testing.T) {
	agg := aggregator.New()
	agg.Add(models.SyscallEvent{Name: "write", Latency: 2 * time.Millisecond, Time: time.Now()})
	ctrl := fakeCtrl{w: 120, h: 40, agg: agg, started: time.Now(), target: "proc"}
	out := RenderView(ctrl)
	if out == "" {
		t.Fatalf("RenderView returned empty string")
	}
	if !strings.Contains(out, "syscalls:") {
		t.Fatalf("RenderView output missing stats header\nOutput: %s", out)
	}
}
