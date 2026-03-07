package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/fabianoflorentino/stracectl/internal/aggregator"
	"github.com/fabianoflorentino/stracectl/internal/models"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func newTestModel() model {
	return model{
		agg:     aggregator.New(),
		sortBy:  aggregator.SortByCount,
		started: time.Now(),
		width:   120,
		height:  40,
	}
}

func addEvent(agg *aggregator.Aggregator, name string, latency time.Duration, errName string) {
	agg.Add(models.SyscallEvent{
		PID:     1,
		Name:    name,
		Latency: latency,
		Error:   errName,
		Time:    time.Now(),
	})
}

func pressKey(m model, key string) model {
	var msg tea.KeyMsg
	switch key {
	case "up":
		msg = tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		msg = tea.KeyMsg{Type: tea.KeyDown}
	case "esc":
		msg = tea.KeyMsg{Type: tea.KeyEscape}
	case "enter":
		msg = tea.KeyMsg{Type: tea.KeyEnter}
	case "backspace":
		msg = tea.KeyMsg{Type: tea.KeyBackspace}
	default:
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
	next, _ := m.Update(msg)
	return next.(model)
}

// ── wordWrap ──────────────────────────────────────────────────────────────────

func TestWordWrap_ShortText(t *testing.T) {
	lines := wordWrap("hello world", 80)
	if len(lines) != 1 || lines[0] != "hello world" {
		t.Errorf("got %v, want [\"hello world\"]", lines)
	}
}

func TestWordWrap_ExactWidth(t *testing.T) {
	text := "abc def"
	lines := wordWrap(text, len(text))
	if len(lines) != 1 {
		t.Errorf("expected 1 line, got %d: %v", len(lines), lines)
	}
}

func TestWordWrap_WrapsLongText(t *testing.T) {
	text := "one two three four five six seven eight nine ten"
	lines := wordWrap(text, 20)
	if len(lines) < 2 {
		t.Errorf("expected multiple lines, got %d: %v", len(lines), lines)
	}
	for _, l := range lines {
		if len(l) > 20 {
			t.Errorf("line too long (%d chars): %q", len(l), l)
		}
	}
}

func TestWordWrap_ZeroWidth(t *testing.T) {
	// maxWidth <= 0 returns the text as-is
	lines := wordWrap("hello world", 0)
	if len(lines) != 1 || lines[0] != "hello world" {
		t.Errorf("got %v", lines)
	}
}

func TestWordWrap_SingleLongWord(t *testing.T) {
	// a word longer than maxWidth must not be split (just placed on its own line)
	lines := wordWrap("superlongwordthatexceedslimit", 5)
	if len(lines) != 1 || lines[0] != "superlongwordthatexceedslimit" {
		t.Errorf("got %v", lines)
	}
}

// ── formatDur ─────────────────────────────────────────────────────────────────

func TestFormatDur_Zero(t *testing.T) {
	if got := formatDur(0); got != "—" {
		t.Errorf("got %q, want \"—\"", got)
	}
}

func TestFormatDur_Nanoseconds(t *testing.T) {
	got := formatDur(500 * time.Nanosecond)
	if !strings.HasSuffix(got, "ns") {
		t.Errorf("got %q, expected suffix ns", got)
	}
}

func TestFormatDur_Microseconds(t *testing.T) {
	got := formatDur(42_500 * time.Nanosecond)
	if !strings.HasSuffix(got, "µs") {
		t.Errorf("got %q, expected suffix µs", got)
	}
}

func TestFormatDur_Milliseconds(t *testing.T) {
	got := formatDur(7_200 * time.Microsecond)
	if !strings.HasSuffix(got, "ms") {
		t.Errorf("got %q, expected suffix ms", got)
	}
}

func TestFormatDur_Seconds(t *testing.T) {
	got := formatDur(2 * time.Second)
	if !strings.HasSuffix(got, "s") {
		t.Errorf("got %q, expected suffix s", got)
	}
}

// ── formatCount ───────────────────────────────────────────────────────────────

func TestFormatCount_Small(t *testing.T) {
	if got := formatCount(42); got != "42" {
		t.Errorf("got %q", got)
	}
}

func TestFormatCount_Thousands(t *testing.T) {
	got := formatCount(1_500)
	if !strings.HasSuffix(got, "k") {
		t.Errorf("got %q, expected 'k' suffix", got)
	}
}

func TestFormatCount_Millions(t *testing.T) {
	got := formatCount(2_300_000)
	if !strings.HasSuffix(got, "M") {
		t.Errorf("got %q, expected 'M' suffix", got)
	}
}

// ── alertExplanation ─────────────────────────────────────────────────────────

func TestAlertExplanation_KnownSyscalls(t *testing.T) {
	known := []string{
		"ioctl", "openat", "open", "access", "faccessat",
		"connect", "recvfrom", "recv", "recvmsg",
		"sendto", "send", "sendmsg",
		"madvise", "prctl", "statfs", "fstatfs",
		"unlink", "unlinkat", "mkdir", "mkdirat",
	}
	for _, name := range known {
		if got := alertExplanation(name); got == "" {
			t.Errorf("alertExplanation(%q) returned empty, want non-empty", name)
		}
	}
}

func TestAlertExplanation_Unknown(t *testing.T) {
	if got := alertExplanation("nonexistentsyscall"); got != "" {
		t.Errorf("got %q, want empty string for unknown syscall", got)
	}
}

// ── syscallInfo ───────────────────────────────────────────────────────────────

func TestSyscallInfo_KnownSyscalls(t *testing.T) {
	known := []string{
		"read", "write", "openat", "close",
		"mmap", "munmap", "mprotect", "madvise",
		"brk", "fstat", "stat",
		"getdents", "access", "connect", "accept",
		"recvfrom", "sendto",
		"epoll_wait", "epoll_ctl", "poll",
		"futex", "clone", "execve", "exit_group",
		"wait4", "ioctl", "prctl",
		"rt_sigaction", "rt_sigprocmask",
		"getpid", "getuid",
		"lseek", "pipe", "dup",
		"socket", "bind", "listen",
		"setsockopt", "getsockname", "getrandom",
		"statfs", "fcntl", "sendfile",
		"prlimit64", "eventfd", "set_tid_address", "arch_prctl",
	}
	for _, name := range known {
		info := syscallInfo(name)
		if info.description == "" {
			t.Errorf("syscallInfo(%q).description is empty", name)
		}
	}
}

func TestSyscallInfo_UnknownSyscall(t *testing.T) {
	info := syscallInfo("totallyfake")
	if info.description == "" {
		t.Error("expected fallback description for unknown syscall, got empty")
	}
	if !strings.Contains(info.notes, "man 2") {
		t.Errorf("expected man-page hint in notes, got %q", info.notes)
	}
}

// ── Cursor navigation ─────────────────────────────────────────────────────────

func TestCursor_StartsAtZero(t *testing.T) {
	m := newTestModel()
	if m.cursor != 0 {
		t.Errorf("initial cursor = %d, want 0", m.cursor)
	}
}

func TestCursor_DownIncrements(t *testing.T) {
	m := newTestModel()
	m = pressKey(m, "j")
	if m.cursor != 1 {
		t.Errorf("cursor after j = %d, want 1", m.cursor)
	}
}

func TestCursor_DownArrowIncrements(t *testing.T) {
	m := newTestModel()
	m = pressKey(m, "down")
	if m.cursor != 1 {
		t.Errorf("cursor after ↓ = %d, want 1", m.cursor)
	}
}

func TestCursor_UpDoesNotGoBelowZero(t *testing.T) {
	m := newTestModel()
	m = pressKey(m, "k")
	if m.cursor != 0 {
		t.Errorf("cursor after k at 0 = %d, want 0 (no underflow)", m.cursor)
	}
}

func TestCursor_UpArrowDoesNotGoBelowZero(t *testing.T) {
	m := newTestModel()
	m = pressKey(m, "up")
	if m.cursor != 0 {
		t.Errorf("cursor after ↑ at 0 = %d, want 0 (no underflow)", m.cursor)
	}
}

func TestCursor_DownThenUp(t *testing.T) {
	m := newTestModel()
	m = pressKey(m, "j")
	m = pressKey(m, "j")
	m = pressKey(m, "k")
	if m.cursor != 1 {
		t.Errorf("cursor after jjk = %d, want 1", m.cursor)
	}
}

func TestCursor_EscResetsToZero(t *testing.T) {
	m := newTestModel()
	m = pressKey(m, "j")
	m = pressKey(m, "j")
	m = pressKey(m, "esc")
	if m.cursor != 0 {
		t.Errorf("cursor after esc = %d, want 0", m.cursor)
	}
}

// ── Detail overlay ────────────────────────────────────────────────────────────

func TestDetailOverlay_DOpens(t *testing.T) {
	m := newTestModel()
	m = pressKey(m, "d")
	if !m.detailOverlay {
		t.Error("expected detailOverlay=true after pressing d")
	}
}

func TestDetailOverlay_UpperDOpens(t *testing.T) {
	m := newTestModel()
	m = pressKey(m, "D")
	if !m.detailOverlay {
		t.Error("expected detailOverlay=true after pressing D")
	}
}

func TestDetailOverlay_AnyKeyCloses(t *testing.T) {
	for _, key := range []string{"d", "q", "j", "esc"} {
		m := newTestModel()
		m.detailOverlay = true
		m = pressKey(m, key)
		if m.detailOverlay {
			t.Errorf("detailOverlay should be false after pressing %q to close, got true", key)
		}
	}
}

func TestDetailOverlay_Renders(t *testing.T) {
	m := newTestModel()
	addEvent(m.agg, "read", 10*time.Microsecond, "")
	addEvent(m.agg, "read", 5*time.Microsecond, "")
	m.detailOverlay = true

	out := m.View()
	if !strings.Contains(out, "read") {
		t.Errorf("renderDetail output does not contain syscall name 'read'\nOutput: %s", out)
	}
	if !strings.Contains(out, "SYSCALL REFERENCE") {
		t.Errorf("renderDetail output missing 'SYSCALL REFERENCE' section\nOutput: %s", out)
	}
	if !strings.Contains(out, "LIVE STATISTICS") {
		t.Errorf("renderDetail output missing 'LIVE STATISTICS' section\nOutput: %s", out)
	}
}

func TestDetailOverlay_EmptyAgg(t *testing.T) {
	m := newTestModel()
	m.detailOverlay = true
	out := m.View()
	// Should not panic; must return something
	if out == "" {
		t.Error("renderDetail returned empty string for empty aggregator")
	}
}

// ── Help overlay ──────────────────────────────────────────────────────────────

func TestHelpOverlay_QuestionMarkOpens(t *testing.T) {
	m := newTestModel()
	m = pressKey(m, "?")
	if !m.helpOverlay {
		t.Error("expected helpOverlay=true after pressing ?")
	}
}

func TestHelpOverlay_AnyKeyCloses(t *testing.T) {
	m := newTestModel()
	m.helpOverlay = true
	m = pressKey(m, "j")
	if m.helpOverlay {
		t.Error("helpOverlay should be false after any key")
	}
}

// ── Sort key bindings ─────────────────────────────────────────────────────────

func TestSortKeys(t *testing.T) {
	cases := []struct {
		key  string
		want aggregator.SortField
	}{
		{"c", aggregator.SortByCount},
		{"t", aggregator.SortByTotal},
		{"a", aggregator.SortByAvg},
		{"x", aggregator.SortByMax},
		{"e", aggregator.SortByErrors},
		{"n", aggregator.SortByName},
	}
	for _, tc := range cases {
		m := newTestModel()
		m = pressKey(m, tc.key)
		if m.sortBy != tc.want {
			t.Errorf("key %q: sortBy = %v, want %v", tc.key, m.sortBy, tc.want)
		}
	}
}

// ── Filter mode ───────────────────────────────────────────────────────────────

func TestFilterMode_SlashEntersEditing(t *testing.T) {
	m := newTestModel()
	m = pressKey(m, "/")
	if !m.editing {
		t.Error("expected editing=true after pressing /")
	}
	if m.filter != "" {
		t.Errorf("filter should be empty on enter, got %q", m.filter)
	}
}

func TestFilterMode_TypeAndBackspace(t *testing.T) {
	m := newTestModel()
	m = pressKey(m, "/")
	m = pressKey(m, "r")
	m = pressKey(m, "e")
	m = pressKey(m, "a")
	if m.filter != "rea" {
		t.Errorf("filter = %q, want \"rea\"", m.filter)
	}
	m = pressKey(m, "backspace")
	if m.filter != "re" {
		t.Errorf("after backspace filter = %q, want \"re\"", m.filter)
	}
}

func TestFilterMode_EscapeExits(t *testing.T) {
	m := newTestModel()
	m = pressKey(m, "/")
	m = pressKey(m, "r")
	m = pressKey(m, "esc")
	if m.editing {
		t.Error("expected editing=false after esc")
	}
}

func TestFilterMode_EnterExits(t *testing.T) {
	m := newTestModel()
	m = pressKey(m, "/")
	m = pressKey(m, "r")
	m = pressKey(m, "enter")
	if m.editing {
		t.Error("expected editing=false after enter")
	}
	if m.filter != "r" {
		t.Errorf("filter should be retained after enter, got %q", m.filter)
	}
}

// ── View smoke test ───────────────────────────────────────────────────────────

func TestView_MainTableRendersRows(t *testing.T) {
	m := newTestModel()
	addEvent(m.agg, "read", 10*time.Microsecond, "")
	addEvent(m.agg, "write", 20*time.Microsecond, "")
	addEvent(m.agg, "openat", 5*time.Microsecond, "ENOENT")

	out := m.View()
	for _, name := range []string{"read", "write", "openat"} {
		if !strings.Contains(out, name) {
			t.Errorf("View() output missing syscall %q", name)
		}
	}
}

func TestView_CursorMarkerVisible(t *testing.T) {
	m := newTestModel()
	addEvent(m.agg, "read", 10*time.Microsecond, "")
	addEvent(m.agg, "write", 10*time.Microsecond, "")
	// cursor=0 by default → first row should have ► marker
	out := m.View()
	if !strings.Contains(out, "►") {
		t.Error("View() output missing cursor marker ►")
	}
}

func TestView_Initialising(t *testing.T) {
	m := newTestModel()
	m.width = 0
	out := m.View()
	if !strings.Contains(out, "Initialising") {
		t.Errorf("View() with width=0 should show initialising message, got %q", out)
	}
}

// ── windowSizeMsg ─────────────────────────────────────────────────────────────

func TestUpdate_WindowSize(t *testing.T) {
	m := newTestModel()
	next, _ := m.Update(tea.WindowSizeMsg{Width: 160, Height: 50})
	got := next.(model)
	if got.width != 160 || got.height != 50 {
		t.Errorf("width=%d height=%d, want 160 50", got.width, got.height)
	}
}

// ── renderSummary health indicators ──────────────────────────────────────────

func TestRenderSummary_WaitingWhenEmpty(t *testing.T) {
	m := newTestModel()
	out := m.View()
	if !strings.Contains(out, "Waiting") {
		t.Errorf("View() with no events should contain 'Waiting', got:\n%s", out)
	}
}

func TestRenderSummary_NoErrors(t *testing.T) {
	m := newTestModel()
	addEvent(m.agg, "read", 1*time.Millisecond, "")
	out := m.View()
	if !strings.Contains(out, "no errors") {
		t.Errorf("expected 'no errors' indicator, got:\n%s", out)
	}
}

func TestRenderSummary_LowErrorRate(t *testing.T) {
	m := newTestModel()
	// 1 error out of 10 = 10% → "likely normal"
	for i := 0; i < 9; i++ {
		addEvent(m.agg, "read", 1*time.Millisecond, "")
	}
	addEvent(m.agg, "read", 1*time.Millisecond, "ENOENT")
	out := m.View()
	if !strings.Contains(out, "likely normal") {
		t.Errorf("expected 'likely normal' for 10%% error rate, got:\n%s", out)
	}
}

func TestRenderSummary_MediumErrorRate(t *testing.T) {
	m := newTestModel()
	// 2 errors out of 8 = 25% → "worth investigating"
	for i := 0; i < 6; i++ {
		addEvent(m.agg, "read", 1*time.Millisecond, "")
	}
	addEvent(m.agg, "read", 1*time.Millisecond, "EIO")
	addEvent(m.agg, "read", 1*time.Millisecond, "EIO")
	out := m.View()
	if !strings.Contains(out, "worth investigating") {
		t.Errorf("expected 'worth investigating' for 25%% error rate, got:\n%s", out)
	}
}

func TestRenderSummary_HighErrorRate(t *testing.T) {
	m := newTestModel()
	// 5 errors out of 6 = 83% → "high, check alerts"
	addEvent(m.agg, "read", 1*time.Millisecond, "")
	for i := 0; i < 5; i++ {
		addEvent(m.agg, "read", 1*time.Millisecond, "EIO")
	}
	out := m.View()
	if !strings.Contains(out, "high") {
		t.Errorf("expected 'high' error indicator for 83%% error rate, got:\n%s", out)
	}
}

func TestRenderSummary_TwoDominantCategories(t *testing.T) {
	m := newTestModel()
	// FS category: stat (many)
	for i := 0; i < 20; i++ {
		addEvent(m.agg, "stat", 1*time.Millisecond, "")
	}
	// NET category: connect (enough to show second category ≥10%)
	for i := 0; i < 5; i++ {
		addEvent(m.agg, "connect", 1*time.Millisecond, "")
	}
	out := m.View()
	if !strings.Contains(out, "then") {
		t.Errorf("expected 'then' second-category clause for two dominant categories, got:\n%s", out)
	}
}

// ── renderAlerts ─────────────────────────────────────────────────────────────

func TestRenderAlerts_SlowSyscall(t *testing.T) {
	m := newTestModel()
	// avg latency > slowAvgThreshold (5ms)
	addEvent(m.agg, "futex", 10*time.Millisecond, "")
	addEvent(m.agg, "futex", 10*time.Millisecond, "")
	out := m.View()
	if !strings.Contains(out, "futex") {
		t.Errorf("expected slow-syscall alert for futex, got:\n%s", out)
	}
}

func TestRenderAlerts_HotErrorRow(t *testing.T) {
	m := newTestModel()
	// ERR% >= 50: all calls fail
	addEvent(m.agg, "ioctl", 1*time.Microsecond, "EINVAL")
	addEvent(m.agg, "ioctl", 1*time.Microsecond, "EINVAL")
	out := m.View()
	if !strings.Contains(out, "ioctl") {
		t.Errorf("expected hot-error alert for ioctl, got:\n%s", out)
	}
}

// ── renderHelp ────────────────────────────────────────────────────────────────

func TestRenderHelp_ContainsExpectedSections(t *testing.T) {
	m := newTestModel()
	m.helpOverlay = true
	out := m.View()
	for _, section := range []string{"COLUMNS", "ROW COLOURS", "CATEGORY BAR", "KEYBOARD SHORTCUTS"} {
		if !strings.Contains(out, section) {
			t.Errorf("renderHelp missing section %q", section)
		}
	}
}

func TestRenderHelp_ContainsKeyBindings(t *testing.T) {
	m := newTestModel()
	m.helpOverlay = true
	out := m.View()
	for _, key := range []string{"q", "?", "/"} {
		if !strings.Contains(out, key) {
			t.Errorf("renderHelp missing key %q", key)
		}
	}
}

// ── sparkBar ─────────────────────────────────────────────────────────────────

func TestSparkBar_ZeroMax(t *testing.T) {
	out := sparkBar(0, 0, 10)
	if !strings.Contains(out, "░") || strings.Contains(out, "█") {
		t.Errorf("sparkBar with maxCount=0 should be all empty, got %q", out)
	}
}

func TestSparkBar_FullBar(t *testing.T) {
	out := sparkBar(10, 10, 8)
	if out != "████████" {
		t.Errorf("sparkBar full fill = %q, want all █", out)
	}
}

func TestSparkBar_HalfBar(t *testing.T) {
	out := sparkBar(5, 10, 10)
	// Half filled: 5 filled + 5 empty = 10 bar characters total
	filled := strings.Count(out, "█")
	empty := strings.Count(out, "░")
	if filled != 5 || empty != 5 {
		t.Errorf("sparkBar(5,10,10): filled=%d empty=%d, want filled=5 empty=5", filled, empty)
	}
}

func TestSparkBar_ZeroWidth(t *testing.T) {
	out := sparkBar(5, 10, 0)
	if out != "" {
		t.Errorf("sparkBar with width=0 should be empty, got %q", out)
	}
}

// ── catStyle ─────────────────────────────────────────────────────────────────

func TestCatStyle_AllCategories(t *testing.T) {
	m := newTestModel()
	// Add one syscall from each category and render — catStyle must not panic
	syscalls := []string{
		"read",         // IO
		"stat",         // FS
		"connect",      // Net
		"mmap",         // Mem
		"clone",        // Process
		"rt_sigaction", // Signal
		"ioctl",        // Other
	}
	for _, name := range syscalls {
		addEvent(m.agg, name, 1*time.Microsecond, "")
	}
	// View must not panic and must contain all names
	out := m.View()
	for _, name := range syscalls {
		if !strings.Contains(out, name) {
			t.Errorf("View() missing syscall %q in output", name)
		}
	}
}

// ── padR / padL truncation ────────────────────────────────────────────────────

func TestPadR_Truncates(t *testing.T) {
	out := padR("toolongstring", 5)
	if len(out) != 5 {
		t.Errorf("padR truncated len = %d, want 5", len(out))
	}
}

func TestPadR_Pads(t *testing.T) {
	out := padR("hi", 6)
	if len(out) != 6 {
		t.Errorf("padR padded len = %d, want 6", len(out))
	}
}

func TestPadL_Truncates(t *testing.T) {
	out := padL("toolongstring", 5)
	if len(out) != 5 {
		t.Errorf("padL truncated len = %d, want 5", len(out))
	}
}

func TestPadL_Pads(t *testing.T) {
	out := padL("hi", 6)
	if len(out) != 6 {
		t.Errorf("padL padded len = %d, want 6", len(out))
	}
}

// ── renderDetail with active filter ──────────────────────────────────────────

func TestDetailOverlay_WithFilter(t *testing.T) {
	m := newTestModel()
	addEvent(m.agg, "read", 10*time.Microsecond, "")
	addEvent(m.agg, "write", 5*time.Microsecond, "")
	m.filter = "wr" // only "write" matches
	m.detailOverlay = true
	out := m.View()
	if !strings.Contains(out, "write") {
		t.Errorf("detail overlay with filter 'wr' should show 'write', got:\n%s", out)
	}
}

func TestDetailOverlay_AnomalySection(t *testing.T) {
	m := newTestModel()
	// ERR% >= 50 → anomaly explanation block should appear
	addEvent(m.agg, "connect", 1*time.Microsecond, "ECONNREFUSED")
	addEvent(m.agg, "connect", 1*time.Microsecond, "ECONNREFUSED")
	m.detailOverlay = true
	out := m.View()
	if !strings.Contains(out, "ANOMALY") {
		t.Errorf("detail overlay should show ANOMALY EXPLANATION for 100%% error rate, got:\n%s", out)
	}
}

// ── Update tickMsg ────────────────────────────────────────────────────────────

func TestUpdate_TickMsg_ReturnsTick(t *testing.T) {
	m := newTestModel()
	_, cmd := m.Update(tickMsg(time.Now()))
	if cmd == nil {
		t.Error("Update(tickMsg) should return a non-nil tick command")
	}
}

// ── View with helpOverlay width=0 ────────────────────────────────────────────

func TestRenderHelp_ZeroWidthFallback(t *testing.T) {
	m := newTestModel()
	m.width = 0
	m.helpOverlay = true
	// Should not panic even with zero width
	out := m.View()
	if out == "" {
		t.Error("renderHelp with width=0 returned empty string")
	}
}
