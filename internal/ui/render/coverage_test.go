package render

import (
	"strings"
	"testing"
	"time"

	"github.com/fabianoflorentino/stracectl/internal/aggregator"
	"github.com/fabianoflorentino/stracectl/internal/models"
	"github.com/fabianoflorentino/stracectl/internal/procinfo"
)

// ---------------------------------------------------------------------------
// formatDurShort
// ---------------------------------------------------------------------------

func TestFormatDurShort_ZeroDuration(t *testing.T) {
	got := formatDurShort(0)
	if got != "—" {
		t.Errorf("expected \"—\" for zero duration, got %q", got)
	}
}

func TestFormatDurShort_NonZero(t *testing.T) {
	d := 3 * time.Millisecond
	got := formatDurShort(d)
	if got != d.String() {
		t.Errorf("expected %q, got %q", d.String(), got)
	}
}

// ---------------------------------------------------------------------------
// RenderDetail – missing branches
// ---------------------------------------------------------------------------

func TestRenderDetail_EmptyAgg_ReturnsNoSyscallSelected(t *testing.T) {
	agg := aggregator.New()
	out := RenderDetail(agg, aggregator.SortByCount, "", 0, 80, 24)
	if !strings.Contains(out, "no syscall selected") {
		t.Errorf("expected 'no syscall selected', got: %q", out)
	}
}

func TestRenderDetail_ZeroWidth_DefaultsTo80(t *testing.T) {
	agg := aggregator.New()
	agg.Add(models.SyscallEvent{Name: "read", Latency: 1 * time.Microsecond, Time: time.Now()})
	// w=0 should not panic and should default to 80
	out := RenderDetail(agg, aggregator.SortByCount, "", 0, 0, 24)
	if out == "" {
		t.Error("RenderDetail with w=0 returned empty string")
	}
}

func TestRenderDetail_FilterMatchesNothing_ReturnsNoSyscallSelected(t *testing.T) {
	agg := aggregator.New()
	agg.Add(models.SyscallEvent{Name: "read", Latency: 1 * time.Microsecond, Time: time.Now()})
	out := RenderDetail(agg, aggregator.SortByCount, "zzznomatch", 0, 80, 24)
	if !strings.Contains(out, "no syscall selected") {
		t.Errorf("expected 'no syscall selected' for non-matching filter, got: %q", out)
	}
}

func TestRenderDetail_FilterMatchesSyscall(t *testing.T) {
	agg := aggregator.New()
	agg.Add(models.SyscallEvent{Name: "read", Latency: 1 * time.Microsecond, Time: time.Now()})
	agg.Add(models.SyscallEvent{Name: "write", Latency: 1 * time.Microsecond, Time: time.Now()})
	out := RenderDetail(agg, aggregator.SortByCount, "rea", 0, 80, 24)
	if !strings.Contains(out, "read") {
		t.Errorf("expected 'read' in filtered detail, got: %q", out)
	}
}

func TestRenderDetail_CursorBeyondStats_Clamped(t *testing.T) {
	agg := aggregator.New()
	agg.Add(models.SyscallEvent{Name: "read", Latency: 1 * time.Microsecond, Time: time.Now()})
	// cursor=999 should clamp to last index
	out := RenderDetail(agg, aggregator.SortByCount, "", 999, 80, 24)
	if !strings.Contains(out, "read") {
		t.Errorf("expected 'read' after cursor clamp, got: %q", out)
	}
}

func TestRenderDetail_WithErrors_ShowsErrorBreakdown(t *testing.T) {
	agg := aggregator.New()
	now := time.Now()
	for i := 0; i < 3; i++ {
		agg.Add(models.SyscallEvent{Name: "read", Latency: 1 * time.Millisecond, Time: now})
	}
	for i := 0; i < 7; i++ {
		agg.Add(models.SyscallEvent{Name: "read", Latency: 1 * time.Millisecond, Time: now, Error: "EAGAIN"})
	}
	out := RenderDetail(agg, aggregator.SortByCount, "", 0, 80, 24)
	if !strings.Contains(out, "ERROR BREAKDOWN") {
		t.Errorf("expected ERROR BREAKDOWN section with errors, got: %q", out)
	}
}

func TestRenderDetail_WithRecentErrors_ShowsRecentErrorSamples(t *testing.T) {
	agg := aggregator.New()
	now := time.Now()
	// add multiple error samples to populate RecentErrors
	for i := 0; i < 3; i++ {
		agg.Add(models.SyscallEvent{
			Name:    "read",
			Latency: 1 * time.Millisecond,
			Time:    now,
			Error:   "EBADF",
			Args:    "fd=5",
		})
	}
	out := RenderDetail(agg, aggregator.SortByCount, "", 0, 80, 24)
	if !strings.Contains(out, "RECENT ERROR SAMPLES") {
		t.Errorf("expected RECENT ERROR SAMPLES section, got: %q", out)
	}
}

func TestRenderDetail_RecentErrorLongArgs_Truncated(t *testing.T) {
	agg := aggregator.New()
	now := time.Now()
	longArgs := strings.Repeat("x", 200)
	agg.Add(models.SyscallEvent{
		Name:    "read",
		Latency: 1 * time.Millisecond,
		Time:    now,
		Error:   "EBADF",
		Args:    longArgs,
	})
	out := RenderDetail(agg, aggregator.SortByCount, "", 0, 80, 24)
	if !strings.Contains(out, "RECENT ERROR SAMPLES") {
		t.Errorf("expected RECENT ERROR SAMPLES section, got: %q", out)
	}
}

func TestRenderDetail_AnomalyExplanation_ShownWhenHighErrPct(t *testing.T) {
	agg := aggregator.New()
	now := time.Now()
	for i := 0; i < 2; i++ {
		agg.Add(models.SyscallEvent{Name: "openat", Latency: 1 * time.Millisecond, Time: now})
	}
	for i := 0; i < 8; i++ {
		agg.Add(models.SyscallEvent{Name: "openat", Latency: 1 * time.Millisecond, Time: now, Error: "ENOENT"})
	}
	out := RenderDetail(agg, aggregator.SortByCount, "", 0, 80, 24)
	if !strings.Contains(out, "ANOMALY EXPLANATION") {
		t.Errorf("expected ANOMALY EXPLANATION section for openat with 80%% errors, got: %q", out)
	}
}

func TestRenderDetail_NarrowWidth_WrapWidthClamped(t *testing.T) {
	// w=30 → wrapWidth = 30-22 = 8 < 40, so it clamps to 40
	// Use "read" which has Notes populated so the wrap logic is exercised
	agg := aggregator.New()
	agg.Add(models.SyscallEvent{Name: "read", Latency: 1 * time.Microsecond, Time: time.Now()})
	out := RenderDetail(agg, aggregator.SortByCount, "", 0, 30, 24)
	if out == "" {
		t.Error("RenderDetail with narrow width returned empty string")
	}
}

// ---------------------------------------------------------------------------
// RenderView – missing branches
// ---------------------------------------------------------------------------

func TestRenderView_ZeroWidth_ReturnsInitMessage(t *testing.T) {
	agg := aggregator.New()
	ctrl := fakeCtrl{w: 0, h: 40, agg: agg, started: time.Now(), target: "proc"}
	out := RenderView(ctrl)
	if !strings.Contains(out, "Initialising stracectl") {
		t.Errorf("expected init message for w=0, got: %q", out)
	}
}

func TestRenderView_ProcInfoComm_ShowsCommAndPID(t *testing.T) {
	agg := aggregator.New()
	agg.SetProcInfo(procinfo.ProcInfo{Comm: "myapp", PID: 1234})
	agg.Add(models.SyscallEvent{Name: "read", Latency: 1 * time.Microsecond, Time: time.Now()})
	ctrl := fakeCtrl{w: 120, h: 40, agg: agg, started: time.Now(), target: "myapp"}
	out := RenderView(ctrl)
	if !strings.Contains(out, "myapp[1234]") {
		t.Errorf("expected 'myapp[1234]' in proc label, got: %q", out)
	}
}

func TestRenderView_SmallWidth_GapClamped(t *testing.T) {
	// Width of 30 forces gap < 0 → clamped to 0, no panic
	agg := aggregator.New()
	ctrl := fakeCtrl{w: 30, h: 40, agg: agg, started: time.Now(), target: "proc"}
	out := RenderView(ctrl)
	if out == "" {
		t.Error("RenderView with small width returned empty string")
	}
}

func TestRenderView_EditingMode_ShowsFilterPrompt(t *testing.T) {
	agg := aggregator.New()
	agg.Add(models.SyscallEvent{Name: "read", Latency: 1 * time.Microsecond, Time: time.Now()})
	ctrl := fakeCtrl{
		w: 120, h: 40, agg: agg, started: time.Now(),
		target: "proc", editing: true, filter: "rea",
	}
	out := RenderView(ctrl)
	if !strings.Contains(out, "filter:") {
		t.Errorf("expected filter prompt in editing mode, got: %q", out)
	}
}

func TestRenderView_ProcessDone_ShowsFooter(t *testing.T) {
	agg := aggregator.New()
	agg.Add(models.SyscallEvent{Name: "read", Latency: 1 * time.Microsecond, Time: time.Now()})
	ctrl := fakeCtrl{
		w: 120, h: 40, agg: agg, started: time.Now(),
		target: "proc", done: true,
	}
	out := RenderView(ctrl)
	if out == "" {
		t.Error("RenderView with ProcessDone=true returned empty string")
	}
}

func TestRenderView_FilterApplied_ShowsFilterInFooter(t *testing.T) {
	agg := aggregator.New()
	agg.Add(models.SyscallEvent{Name: "read", Latency: 1 * time.Microsecond, Time: time.Now()})
	ctrl := fakeCtrl{
		w: 120, h: 40, agg: agg, started: time.Now(),
		target: "proc", filter: "rea",
	}
	out := RenderView(ctrl)
	if !strings.Contains(out, "filter:") {
		t.Errorf("expected [filter: ...] in footer, got: %q", out)
	}
}

func TestRenderView_AlertsPresent_ShowsAlertHeader(t *testing.T) {
	agg := aggregator.New()
	now := time.Now()
	for i := 0; i < 2; i++ {
		agg.Add(models.SyscallEvent{Name: "openat", Latency: 1 * time.Millisecond, Time: now})
	}
	for i := 0; i < 8; i++ {
		agg.Add(models.SyscallEvent{Name: "openat", Latency: 1 * time.Millisecond, Time: now, Error: "ENOENT"})
	}
	ctrl := fakeCtrl{w: 120, h: 40, agg: agg, started: time.Now(), target: "proc"}
	out := RenderView(ctrl)
	if !strings.Contains(out, "ANOMALY ALERTS") {
		t.Errorf("expected ANOMALY ALERTS header, got: %q", out)
	}
}

func TestRenderView_SmallHeight_MaxRowsClamped(t *testing.T) {
	// Very small height forces maxRows < 1 → clamped to 1
	agg := aggregator.New()
	agg.Add(models.SyscallEvent{Name: "read", Latency: 1 * time.Microsecond, Time: time.Now()})
	ctrl := fakeCtrl{w: 120, h: 2, agg: agg, started: time.Now(), target: "proc"}
	out := RenderView(ctrl)
	if out == "" {
		t.Error("RenderView with h=2 returned empty string")
	}
}

func TestRenderView_SlowRow_RenderedWithSlowStyle(t *testing.T) {
	agg := aggregator.New()
	// avg >= 5ms
	agg.Add(models.SyscallEvent{Name: "write", Latency: 10 * time.Millisecond, Time: time.Now()})
	ctrl := fakeCtrl{w: 120, h: 40, agg: agg, started: time.Now(), target: "proc"}
	out := RenderView(ctrl)
	if !strings.Contains(out, "write") {
		t.Errorf("expected 'write' in slow row output, got: %q", out)
	}
}

func TestRenderView_ErrorRow_RenderedWhenSomeErrors(t *testing.T) {
	agg := aggregator.New()
	now := time.Now()
	// 3 ok, 1 error → ErrPct = 25% (< 50%) → errRowStyle
	for i := 0; i < 3; i++ {
		agg.Add(models.SyscallEvent{Name: "close", Latency: 1 * time.Microsecond, Time: now})
	}
	agg.Add(models.SyscallEvent{Name: "close", Latency: 1 * time.Microsecond, Time: now, Error: "EBADF"})
	ctrl := fakeCtrl{w: 120, h: 40, agg: agg, started: time.Now(), target: "proc"}
	out := RenderView(ctrl)
	if !strings.Contains(out, "close") {
		t.Errorf("expected 'close' in error row output, got: %q", out)
	}
}

func TestRenderView_HotRow_RenderedWhenHighErrPct(t *testing.T) {
	agg := aggregator.New()
	now := time.Now()
	// 1 ok, 4 errors → ErrPct = 80% (>= 50%) → hotRowStyle
	agg.Add(models.SyscallEvent{Name: "close", Latency: 1 * time.Microsecond, Time: now})
	for i := 0; i < 4; i++ {
		agg.Add(models.SyscallEvent{Name: "close", Latency: 1 * time.Microsecond, Time: now, Error: "EBADF"})
	}
	ctrl := fakeCtrl{w: 120, h: 40, agg: agg, started: time.Now(), target: "proc"}
	out := RenderView(ctrl)
	if !strings.Contains(out, "close") {
		t.Errorf("expected 'close' in hot row output, got: %q", out)
	}
}

func TestRenderView_ScrollOffset_ActivatedWhenManySyscalls(t *testing.T) {
	agg := aggregator.New()
	now := time.Now()
	names := []string{
		"read", "write", "openat", "close", "fstat", "mmap",
		"brk", "mprotect", "clone", "futex", "epoll_wait",
		"getdents64", "recvfrom", "sendto", "connect",
	}
	for _, name := range names {
		agg.Add(models.SyscallEvent{Name: name, Latency: 1 * time.Microsecond, Time: now})
	}
	// h=10 → few maxRows, cursor at last entry triggers scroll
	ctrl := fakeCtrl{
		w: 120, h: 10, agg: agg, started: time.Now(),
		target: "proc", cursor: len(names) - 1,
	}
	out := RenderView(ctrl)
	if out == "" {
		t.Error("RenderView with scroll offset returned empty string")
	}
}

func TestRenderView_CursorBeyondLen_Clamped(t *testing.T) {
	agg := aggregator.New()
	agg.Add(models.SyscallEvent{Name: "read", Latency: 1 * time.Microsecond, Time: time.Now()})
	ctrl := fakeCtrl{
		w: 120, h: 40, agg: agg, started: time.Now(),
		target: "proc", cursor: 999,
	}
	out := RenderView(ctrl)
	if !strings.Contains(out, "read") {
		t.Errorf("expected 'read' after cursor clamp, got: %q", out)
	}
}

func TestRenderView_FilterStats_RemovesNonMatchingRows(t *testing.T) {
	agg := aggregator.New()
	now := time.Now()
	agg.Add(models.SyscallEvent{Name: "read", Latency: 1 * time.Microsecond, Time: now})
	agg.Add(models.SyscallEvent{Name: "write", Latency: 1 * time.Microsecond, Time: now})
	ctrl := fakeCtrl{
		w: 120, h: 40, agg: agg, started: time.Now(),
		target: "proc", filter: "rea",
	}
	out := RenderView(ctrl)
	if strings.Contains(out, "write") {
		t.Errorf("filtered view should not contain 'write', got: %q", out)
	}
}
