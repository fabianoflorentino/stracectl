package ui

import (
	"io"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/fabianoflorentino/stracectl/internal/aggregator"
	"github.com/fabianoflorentino/stracectl/internal/models"
	"github.com/fabianoflorentino/stracectl/internal/ui/helpers"
	umodel "github.com/fabianoflorentino/stracectl/internal/ui/model"
	"github.com/fabianoflorentino/stracectl/internal/ui/overlays"
	uirender "github.com/fabianoflorentino/stracectl/internal/ui/render"
	"github.com/fabianoflorentino/stracectl/internal/ui/widgets"
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

func addEvent(agg umodel.AggregatorView, name string, latency time.Duration, errName string) {
	// Tests drive the underlying Aggregator; attempt to type-assert to the concrete
	// type used by tests and call Add directly.
	if a, ok := agg.(*aggregator.Aggregator); ok {
		a.Add(models.SyscallEvent{
			PID:     1,
			Name:    name,
			Latency: latency,
			Error:   errName,
			Time:    time.Now(),
		})
		return
	}
	// If the provided AggregatorView is not the concrete Aggregator, tests can
	// not inject events — ignore silently.
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
	nm := next.(*model)
	return *nm
}

// ── wordWrap ──────────────────────────────────────────────────────────────────

func TestWordWrap_ShortText(t *testing.T) {
	lines := widgets.WordWrap("hello world", 80)
	if len(lines) != 1 || lines[0] != "hello world" {
		t.Errorf("got %v, want [\"hello world\"]", lines)
	}
}

func TestWordWrap_ExactWidth(t *testing.T) {
	text := "abc def"
	lines := widgets.WordWrap(text, len(text))
	if len(lines) != 1 {
		t.Errorf("expected 1 line, got %d: %v", len(lines), lines)
	}
}

func TestWordWrap_WrapsLongText(t *testing.T) {
	text := "one two three four five six seven eight nine ten"
	lines := widgets.WordWrap(text, 20)
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
	lines := widgets.WordWrap("hello world", 0)
	if len(lines) != 1 || lines[0] != "hello world" {
		t.Errorf("got %v", lines)
	}
}

func TestWordWrap_SingleLongWord(t *testing.T) {
	// a word longer than maxWidth must not be split (just placed on its own line)
	lines := widgets.WordWrap("superlongwordthatexceedslimit", 5)
	if len(lines) != 1 || lines[0] != "superlongwordthatexceedslimit" {
		t.Errorf("got %v", lines)
	}
}

// ── formatDur ─────────────────────────────────────────────────────────────────

func TestFormatDur_Zero(t *testing.T) {
	if got := helpers.FormatDur(0); got != "—" {
		t.Errorf("got %q, want \"—\"", got)
	}
}

func TestFormatDur_Nanoseconds(t *testing.T) {
	got := helpers.FormatDur(500 * time.Nanosecond)
	if !strings.HasSuffix(got, "ns") {
		t.Errorf("got %q, expected suffix ns", got)
	}
}

func TestFormatDur_Microseconds(t *testing.T) {
	got := helpers.FormatDur(42_500 * time.Nanosecond)
	if !strings.HasSuffix(got, "µs") {
		t.Errorf("got %q, expected suffix µs", got)
	}
}

func TestFormatDur_Milliseconds(t *testing.T) {
	got := helpers.FormatDur(7_200 * time.Microsecond)
	if !strings.HasSuffix(got, "ms") {
		t.Errorf("got %q, expected suffix ms", got)
	}
}

func TestFormatDur_Seconds(t *testing.T) {
	got := helpers.FormatDur(2 * time.Second)
	if !strings.HasSuffix(got, "s") {
		t.Errorf("got %q, expected suffix s", got)
	}
}

// ── formatCount ───────────────────────────────────────────────────────────────

func TestFormatCount_Small(t *testing.T) {
	if got := helpers.FormatCount(42); got != "42" {
		t.Errorf("got %q", got)
	}
}

func TestFormatCount_Thousands(t *testing.T) {
	got := helpers.FormatCount(1_500)
	if !strings.HasSuffix(got, "k") {
		t.Errorf("got %q, expected 'k' suffix", got)
	}
}

func TestFormatCount_Millions(t *testing.T) {
	got := helpers.FormatCount(2_300_000)
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
		if got := uirender.AlertExplanation(name); got == "" {
			t.Errorf("AlertExplanation(%q) returned empty, want non-empty", name)
		}
	}
}

func TestAlertExplanation_Unknown(t *testing.T) {
	if got := uirender.AlertExplanation("nonexistentsyscall"); got != "" {
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

func TestDetailOverlay_NavigationKeys(t *testing.T) {
	// j/k navigate within the detail view, not close it
	for _, key := range []string{"j", "k", "up", "down"} {
		m := newTestModel()
		m.detailOverlay = true
		m = pressKey(m, key)
		if !m.detailOverlay {
			t.Errorf("detailOverlay should remain open after pressing %q (navigation key)", key)
		}
	}
}

func TestDetailOverlay_OtherKeyCloses(t *testing.T) {
	// any non-navigation key closes the detail view (except q which quits)
	for _, key := range []string{"d", "esc", "a", "c", " "} {
		m := newTestModel()
		m.detailOverlay = true
		m = pressKey(m, key)
		if m.detailOverlay {
			t.Errorf("detailOverlay should be false after pressing %q, got true", key)
		}
	}
}

func TestDetailOverlay_EnterOpens(t *testing.T) {
	m := newTestModel()
	addEvent(m.agg, "read", 1*time.Millisecond, "")
	m = pressKey(m, "enter")
	if !m.detailOverlay {
		t.Error("pressing Enter should open detail overlay")
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
		{"g", aggregator.SortByCategory},
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
	got := *(next.(*model))
	if got.width != 160 || got.height != 50 {
		t.Errorf("width=%d height=%d, want 160 50", got.width, got.height)
	}
}

// ── merged title + stats header ───────────────────────────────────────────────

func TestTitleBar_ShowsStats(t *testing.T) {
	m := newTestModel()
	addEvent(m.agg, "read", 1*time.Millisecond, "")
	addEvent(m.agg, "read", 1*time.Millisecond, "ENOENT")
	out := m.View()
	// The single header line should contain both the brand name and live stats.
	for _, want := range []string{"stracectl", "syscalls:", "rate:", "errors:", "unique:"} {
		if !strings.Contains(out, want) {
			t.Errorf("title bar missing %q in:\n%s", want, out)
		}
	}
}

func TestTitleBar_ErrorCountReflected(t *testing.T) {
	m := newTestModel()
	for i := 0; i < 5; i++ {
		addEvent(m.agg, "read", 1*time.Millisecond, "EIO")
	}
	out := m.View()
	// errors count must appear in the header stats.
	if !strings.Contains(out, "errors:") {
		t.Errorf("expected 'errors:' in header, got:\n%s", out)
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
	out := widgets.SparkBar(0, 0, 10)
	if !strings.Contains(out, "░") || strings.Contains(out, "█") {
		t.Errorf("sparkBar with maxCount=0 should be all empty, got %q", out)
	}
}

func TestSparkBar_FullBar(t *testing.T) {
	out := widgets.SparkBar(10, 10, 8)
	if out != "████████" {
		t.Errorf("sparkBar full fill = %q, want all █", out)
	}
}

func TestSparkBar_HalfBar(t *testing.T) {
	out := widgets.SparkBar(5, 10, 10)
	// Half filled: 5 filled + 5 empty = 10 bar characters total
	filled := strings.Count(out, "█")
	empty := strings.Count(out, "░")
	if filled != 5 || empty != 5 {
		t.Errorf("sparkBar(5,10,10): filled=%d empty=%d, want filled=5 empty=5", filled, empty)
	}
}

func TestSparkBar_ZeroWidth(t *testing.T) {
	out := widgets.SparkBar(5, 10, 0)
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

// ── padR / padL ───────────────────────────────────────────────────────────────

func TestPadR_TooLong(t *testing.T) {
	// When the input is longer than n, return it unchanged (no truncation/corruption).
	out := widgets.PadR("toolongstring", 5)
	if out != "toolongstring" {
		t.Errorf("padR too-long: got %q, want unchanged string", out)
	}
}

func TestPadR_Pads(t *testing.T) {
	out := widgets.PadR("hi", 6)
	if len(out) != 6 {
		t.Errorf("padR padded len = %d, want 6", len(out))
	}
}

func TestPadR_MultibytePads(t *testing.T) {
	// "µs" visual width = 2, padded to 5 → should add 3 spaces, total visual 5.
	out := widgets.PadR("µs", 5)
	if lipgloss.Width(out) != 5 {
		t.Errorf("padR multibyte visual width = %d, want 5", lipgloss.Width(out))
	}
}

func TestPadL_TooLong(t *testing.T) {
	// When the input is longer than n, return it unchanged.
	out := widgets.PadL("toolongstring", 5)
	if out != "toolongstring" {
		t.Errorf("padL too-long: got %q, want unchanged string", out)
	}
}

func TestPadL_Pads(t *testing.T) {
	out := widgets.PadL("hi", 6)
	if len(out) != 6 {
		t.Errorf("padL padded len = %d, want 6", len(out))
	}
}

func TestPadL_MultibytePads(t *testing.T) {
	// "37.3µs" visual width = 6 (µ is 2 bytes but 1 column).
	// padL to 10 should prepend 4 spaces, total visual width 10.
	out := widgets.PadL("37.3µs", 10)
	if lipgloss.Width(out) != 10 {
		t.Errorf("padL multibyte visual width = %d, want 10", lipgloss.Width(out))
	}
}

func TestPadL_EmDashPads(t *testing.T) {
	// "—" is 3 bytes but 1 visible column; padL to 8 should produce 7 spaces + —.
	out := widgets.PadL("—", 8)
	if lipgloss.Width(out) != 8 {
		t.Errorf("padL em-dash visual width = %d, want 8", lipgloss.Width(out))
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

// ── Init ──────────────────────────────────────────────────────────────────────

func TestModel_Init_ReturnsCmd(t *testing.T) {
	m := newTestModel()
	cmd := m.Init()
	if cmd == nil {
		t.Error("Init() should return a non-nil tick command")
	}
}

// ── handleKey q/Q inside detailOverlay ───────────────────────────────────────

func TestDetailOverlay_QQuits(t *testing.T) {
	for _, key := range []string{"q", "Q"} {
		m := newTestModel()
		m.detailOverlay = true
		_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
		if cmd == nil {
			t.Errorf("pressing %q in detailOverlay should return tea.Quit cmd, got nil", key)
		}
	}
}

// ── renderHelp w=0 branch (called directly, not through View) ────────────────

func TestRenderHelp_ZeroWidthDirect(t *testing.T) {
	m := newTestModel()
	m.width = 0
	out := overlays.RenderHelp(m.width)
	if out == "" {
		t.Error("renderHelp() with width=0 returned empty string")
	}
}

// ── colWidths name < 14 clamp ─────────────────────────────────────────────────

func TestColWidths_NarrowTerminalClampsName(t *testing.T) {
	// width=50: computed name = 50-75 = -25, clamped to 14
	cw := widgets.ColWidths(50)
	if cw.Name != 14 {
		t.Errorf("colWidths(50).Name = %d, want 14 (clamped)", cw.Name)
	}
}

// ── sparkBar filled > width clamp ────────────────────────────────────────────

func TestSparkBar_CountExceedsMax(t *testing.T) {
	// Passing count > maxCount should not produce more █ than width.
	out := widgets.SparkBar(20, 10, 5)
	if len([]rune(out)) != 5 {
		t.Errorf("sparkBar len = %d, want 5", len([]rune(out)))
	}
	if strings.Count(out, "█") > 5 {
		t.Errorf("sparkBar should not overflow width, got %q", out)
	}
}

// ── View filter path ─────────────────────────────────────────────────────────

func TestView_FilterNarrowsRows(t *testing.T) {
	m := newTestModel()
	addEvent(m.agg, "read", 1*time.Microsecond, "")
	addEvent(m.agg, "write", 1*time.Microsecond, "")
	m.filter = "read"
	out := m.View()
	if !strings.Contains(out, "read") {
		t.Errorf("filtered view should contain 'read', got:\n%s", out)
	}
}

// ── View scroll path ─────────────────────────────────────────────────────────

func TestView_ScrollOffset(t *testing.T) {
	m := newTestModel()
	// Add more rows than maxRows (height=40, fixedLines≈8 → maxRows≈32)
	// Use unique names to exceed maxRows.
	names := []string{
		"read", "write", "openat", "close", "mmap", "munmap", "mprotect",
		"madvise", "brk", "fstat", "getdents64", "access", "connect", "accept4",
		"recvfrom", "sendto", "epoll_wait", "epoll_ctl", "poll", "futex",
		"clone", "execve", "exit_group", "wait4", "ioctl", "prctl",
		"rt_sigaction", "rt_sigprocmask", "getpid", "getuid", "lseek", "pipe",
		"dup", "socket", "bind", "listen",
	}
	for _, name := range names {
		addEvent(m.agg, name, 1*time.Microsecond, "")
	}
	// Push cursor past the first page so scrollOffset > 0.
	m.cursor = len(names) - 1
	out := m.View()
	if out == "" {
		t.Error("View() with scroll offset returned empty string")
	}
}

// ── View gap < 0 path ────────────────────────────────────────────────────────

func TestView_GapNegativeIsClamped(t *testing.T) {
	m := newTestModel()
	// Use a very wide target name + tiny width so gap would go negative.
	m.target = strings.Repeat("x", 200)
	m.width = 40
	// Should not panic, gap is clamped to 0.
	out := m.View()
	if out == "" {
		t.Error("View() with negative gap returned empty string")
	}
}

// ── renderDetail wrapWidth < 40 clamp ────────────────────────────────────────

func TestRenderDetail_NarrowTerminalClampsWrapWidth(t *testing.T) {
	m := newTestModel()
	// width < 62 → wrapWidth = w-22 < 40, triggers clamp to 40
	m.width = 50
	addEvent(m.agg, "read", 1*time.Millisecond, "")
	m.detailOverlay = true
	// Should not panic and must contain the syscall name.
	out := m.View()
	if !strings.Contains(out, "read") {
		t.Errorf("renderDetail narrow width: expected 'read' in output, got:\n%s", out)
	}
}

// ── handleKey q in normal mode (outer switch) ─────────────────────────────────

func TestHandleKey_QQuitsNormalMode(t *testing.T) {
	m := newTestModel()
	// detailOverlay=false, helpOverlay=false — hits the outer switch case
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Error("q in normal mode should return tea.Quit cmd, got nil")
	}
}

// ── handleKey cursor-- inside detailOverlay (cursor > 0) ─────────────────────

func TestDetailOverlay_UpDecrementsWhenCursorPositive(t *testing.T) {
	m := newTestModel()
	m.detailOverlay = true
	m.cursor = 3
	m = pressKey(m, "k")
	if m.cursor != 2 {
		t.Errorf("cursor after 'k' with cursor=3: got %d, want 2", m.cursor)
	}
}

// ── View editing footer path ──────────────────────────────────────────────────

func TestView_EditingFooterShownDuringFilter(t *testing.T) {
	m := newTestModel()
	m.editing = true
	m.filter = "rea"
	addEvent(m.agg, "read", 1*time.Microsecond, "")
	out := m.View()
	if !strings.Contains(out, "filter:") {
		t.Errorf("editing footer not found in View output:\n%s", out)
	}
}

// ── View maxRows clamped to 1 ─────────────────────────────────────────────────

func TestView_MaxRowsClampedToOne(t *testing.T) {
	m := newTestModel()
	m.height = 8 // fixedLines=8, maxRows=0 → clamped to 1
	addEvent(m.agg, "read", 1*time.Microsecond, "")
	out := m.View()
	if out == "" {
		t.Error("View with maxRows clamped to 1 returned empty string")
	}
}

// ── View errRowStyle (errors > 0, ErrPct < hotErrPct) ────────────────────────

func TestView_ErrRowStyleRendered(t *testing.T) {
	m := newTestModel()
	// "write" gets many calls (cursor=0 → selected), "read" gets fewer calls
	// with one error so ErrPct = 33% < 50% = hotErrPct → errRowStyle
	for i := 0; i < 5; i++ {
		addEvent(m.agg, "write", 1*time.Microsecond, "")
	}
	addEvent(m.agg, "read", 1*time.Microsecond, "")
	addEvent(m.agg, "read", 1*time.Microsecond, "")
	addEvent(m.agg, "read", 1*time.Microsecond, "ENOENT") // 33% errors
	m.cursor = 0                                          // cursor on "write", not "read"
	out := m.View()
	if !strings.Contains(out, "read") {
		t.Errorf("errRowStyle row 'read' not found in View output:\n%s", out)
	}
}

// ── View slowRowStyle (avgDur >= slowAvgThreshold, no errors) ─────────────────

func TestView_SlowRowStyleRendered(t *testing.T) {
	m := newTestModel()
	// "read" (cursor=0, selected), "write" (slow, 10ms avg, no errors)
	addEvent(m.agg, "read", 1*time.Microsecond, "")
	addEvent(m.agg, "read", 1*time.Microsecond, "")
	addEvent(m.agg, "write", 10*time.Millisecond, "") // avg=10ms ≥ 5ms slowAvgThreshold
	m.cursor = 0                                      // cursor on "read" (2 calls > 1)
	out := m.View()
	if !strings.Contains(out, "write") {
		t.Errorf("slowRowStyle row 'write' not found in View output:\n%s", out)
	}
}

// ── renderDetail w=0 branch (called directly) ─────────────────────────────────

func TestRenderDetail_ZeroWidthDirect(t *testing.T) {
	m := newTestModel()
	m.width = 0
	addEvent(m.agg, "read", 1*time.Millisecond, "")
	out := uirender.RenderDetail(m.agg, m.sortBy, m.filter, m.cursor, m.width, m.height)
	if !strings.Contains(out, "read") {
		t.Errorf("renderDetail direct w=0: expected 'read' in output, got:\n%s", out)
	}
}

// ── renderDetail cursor clamped to len(stats)-1 ───────────────────────────────

func TestRenderDetail_CursorClampedToLastRow(t *testing.T) {
	m := newTestModel()
	addEvent(m.agg, "read", 1*time.Millisecond, "")
	m.cursor = 99 // well beyond the 1-item stats list
	out := uirender.RenderDetail(m.agg, m.sortBy, m.filter, m.cursor, m.width, m.height)
	if !strings.Contains(out, "read") {
		t.Errorf("renderDetail cursor clamp: expected 'read' in output, got:\n%s", out)
	}
}

// ── Auto-quit when traced process exits ("stuck terminal" regression) ─────────
//
// These tests cover the scenario where the user runs:
//
//	stracectl run ping -c 1 8.8.8.8
//
// and after ping finishes, strace exits, the events channel closes — but the
// TUI used to stay open, leaving the terminal frozen. The fix is that the done
// channel (closed by the aggregator goroutine) causes the TUI to quit via
// processDeadMsg.

// TestProcessDeadMsg_SetsProcessDoneFlag is the pure unit test: the model must
// handle processDeadMsg by setting processDone=true so the footer banner is shown.
// The TUI must NOT quit automatically — the user reviews the data first.
func TestProcessDeadMsg_SetsProcessDoneFlag(t *testing.T) {
	m := newTestModel()
	next, cmd := m.Update(processDeadMsg{})
	got := *(next.(*model))
	if !got.processDone {
		t.Error("Update(processDeadMsg{}) did not set processDone=true")
	}
	if cmd != nil {
		// Should NOT return tea.Quit — user must be able to inspect the final data.
		if msg := cmd(); msg != nil {
			if _, isQuit := msg.(tea.QuitMsg); isQuit {
				t.Error("Update(processDeadMsg{}) returned tea.Quit — TUI must stay open after process exits")
			}
		}
	}
}

// TestProcessDeadMsg_ShowsBanner verifies the footer still shows shortcuts when the process has finished.
// The "process exited" banner was removed — the q shortcut is already visible in the footer.
func TestProcessDeadMsg_ShowsBanner(t *testing.T) {
	m := newTestModel()
	m.processDone = true
	addEvent(m.agg, "read", 1*time.Microsecond, "")
	out := m.View()
	if !strings.Contains(out, "q:quit") {
		t.Errorf("View() with processDone=true should show shortcuts footer, got:\n%s", out)
	}
}

// TestProcessDeadMsg_SetsProcessDoneFlagRegardlessOfState verifies processDone
// is set regardless of which overlay is currently shown.
func TestProcessDeadMsg_SetsProcessDoneFlagRegardlessOfState(t *testing.T) {
	states := []struct {
		name  string
		setup func(*model)
	}{
		{"normal", func(m *model) {}},
		{"helpOverlay", func(m *model) { m.helpOverlay = true }},
		{"detailOverlay", func(m *model) { m.detailOverlay = true }},
		{"editing", func(m *model) { m.editing = true }},
	}
	for _, tc := range states {
		m := newTestModel()
		tc.setup(&m)
		next, _ := m.Update(processDeadMsg{})
		got := *(next.(*model))
		if !got.processDone {
			t.Errorf("[%s] Update(processDeadMsg{}) did not set processDone=true", tc.name)
		}
	}
}

// TestRun_StaysOpenWhenDoneIsClosed is the integration test for the "stuck
// terminal" fix: after done is closed (traced process exited), the TUI must
// stay alive (showing the "process exited" banner) and only exit when the user
// presses q. Without the processDeadMsg handling the user had no indication
// the process had finished; with auto-quit they couldn't see the results.
func TestRun_StaysOpenWhenDoneIsClosed(t *testing.T) {
	agg := aggregator.New()
	done := make(chan struct{})

	pr, pw := io.Pipe()
	t.Cleanup(func() { pw.Close(); pr.Close() })

	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		runWithOpts(agg, "ping -c 1 8.8.8.8", done, nil, //nolint:errcheck
			tea.WithInput(pr),
			tea.WithOutput(io.Discard),
		)
	}()

	// Give the BubbleTea program time to initialise its message loop.
	time.Sleep(50 * time.Millisecond)

	// Simulate the traced process exiting (events channel drained).
	close(done)

	// After done is closed the TUI must NOT auto-quit — it should show the
	// banner and wait for user input.
	time.Sleep(100 * time.Millisecond)
	select {
	case <-stopped:
		t.Fatal("Run auto-quit after done was closed — user cannot inspect final results")
	default:
		// correct: still running, showing banner
	}

	// Now send q: the TUI must exit promptly.
	pw.Write([]byte("q")) //nolint:errcheck
	select {
	case <-stopped:
		// exited cleanly after q
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not exit after q was pressed")
	}
}

// TestRun_NilDoneDoesNotAutoQuit verifies that when done is nil (e.g. stats
// mode reading from a file), the TUI does not quit on its own — it waits for
// explicit user input.
func TestRun_NilDoneDoesNotAutoQuit(t *testing.T) {
	agg := aggregator.New()
	pr, pw := io.Pipe()
	t.Cleanup(func() { pw.Close(); pr.Close() })

	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		runWithOpts(agg, "trace.log", nil, nil, //nolint:errcheck
			tea.WithInput(pr),
			tea.WithOutput(io.Discard),
		)
	}()

	// After 100 ms the TUI must still be running — nil done means no auto-quit.
	// t.Cleanup will close the pipe so BubbleTea's stdin reader unblocks when
	// the test binary exits; we do not need to wait for the goroutine here.
	time.Sleep(100 * time.Millisecond)
	select {
	case <-stopped:
		t.Fatal("Run with nil done quit unexpectedly — stats-file mode would exit immediately after loading")
	default:
		// correct: still running
	}
}

func Test_ModelFromAggregator_DefaultsAndView(t *testing.T) {
	agg := aggregator.New()
	m := ModelFromAggregator(agg, "proc", nil)
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
	m := ModelFromAggregator(agg, "proc", nil)

	// processDeadMsg should mark processDone
	mi, _ := m.Update(processDeadMsg{})
	m2 := *(mi.(*model))
	if !m2.ProcessDone() {
		t.Fatalf("expected ProcessDone after processDeadMsg")
	}

	// tickMsg repeated should eventually set a fallback width/height
	m3 := ModelFromAggregator(agg, "proc", nil)
	for i := 0; i < 6; i++ {
		mi, _ = m3.Update(tickMsg(time.Now()))
		m3 = *(mi.(*model))
	}
	if m3.Width() == 0 || m3.Height() == 0 {
		t.Fatalf("expected fallback size to be set after ticks, got w=%d h=%d", m3.Width(), m3.Height())
	}
}
