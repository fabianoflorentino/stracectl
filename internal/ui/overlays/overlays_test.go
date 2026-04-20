package overlays

import (
	"testing"
	"time"

	"github.com/fabianoflorentino/stracectl/internal/aggregator"
	"github.com/fabianoflorentino/stracectl/internal/procinfo"
)

type fakeAgg struct{}

func (f *fakeAgg) Total() int64                                                        { return 0 }
func (f *fakeAgg) Errors() int64                                                       { return 0 }
func (f *fakeAgg) Rate() float64                                                       { return 0 }
func (f *fakeAgg) UniqueCount() int                                                    { return 0 }
func (f *fakeAgg) Sorted(_ aggregator.SortField) []aggregator.SyscallStat              { return nil }
func (f *fakeAgg) CategoryBreakdown() map[aggregator.Category]aggregator.CategoryStats { return nil }
func (f *fakeAgg) GetProcInfo() procinfo.ProcInfo                                      { return procinfo.ProcInfo{} }
func (f *fakeAgg) IsPerPID() bool                                                      { return false }

func (f *fakeAgg) RecentLog() []aggregator.LogEntry {
	now := time.Now()
	return []aggregator.LogEntry{
		{Time: now, PID: 1, Name: "read", Args: "(3, \"hi\", 2)", Error: ""},
		{Time: now, PID: 1, Name: "openat", Args: "/etc/passwd", Error: "ENOENT"},
	}
}

func (f *fakeAgg) TopFiles(n int) []aggregator.FileStat {
	return []aggregator.FileStat{{Path: "/etc/passwd", Count: 1234}, {Path: "/var/log/syslog", Count: 5}}
}

func TestRenderLog_Basic(t *testing.T) {
	off := 0
	out := RenderLog(80, 10, &fakeAgg{}, &off)
	if out == "" {
		t.Fatal("RenderLog returned empty string")
	}
}

func TestRenderFiles_Basic(t *testing.T) {
	off := 0
	out := RenderFiles(80, 8, &fakeAgg{}, &off)
	if out == "" {
		t.Fatal("RenderFiles returned empty string")
	}
}

func TestRenderHelp_Basic(t *testing.T) {
	out := RenderHelp(80)
	if out == "" {
		t.Fatal("RenderHelp returned empty string")
	}
}
