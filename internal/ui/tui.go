// Package ui provides the BubbleTea TUI for stracectl.
package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/fabianoflorentino/stracectl/internal/aggregator"
)

// refreshInterval is how often the UI updates with new data from the aggregator.
const refreshInterval = 200 * time.Millisecond

// ── Styles ────────────────────────────────────────────────────────────────────
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("255")).
			Background(lipgloss.Color("63"))

	statsStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("248")).
			Background(lipgloss.Color("235"))

	catIOStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("75"))
	catFSStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("149"))
	catNetStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	catMemStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("183"))
	catProcStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("210"))
	catSigStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	catOthStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("220"))

	activeSortStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("118"))

	rowStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	// row with >0 errors but error rate below the warning threshold
	errRowStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("203"))

	// row with very high error rate (>= 50 %)
	hotRowStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("196"))

	// row whose avg latency exceeds the warning threshold
	slowRowStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("227"))

	barFillStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("63"))

	divStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("238"))

	footerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("242"))

	filterStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("229"))

	alertStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("196"))

	selectedRowStyle = lipgloss.NewStyle().
				Bold(true).
				Background(lipgloss.Color("237")).
				Foreground(lipgloss.Color("255"))

	detailTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("255")).
				Background(lipgloss.Color("25"))

	detailLabelStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("220"))

	detailValueStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("252"))

	detailDimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("244"))

	detailCodeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("149"))
)

// ── Thresholds for visual anomaly detection ───────────────────────────────────
const (
	slowAvgThreshold = 5 * time.Millisecond // avg latency considered slow
	hotErrPct        = 50.0                 // % errors considered critical
)

// ── BubbleTea plumbing ────────────────────────────────────────────────────────
type tickMsg time.Time

func tick() tea.Cmd {
	return tea.Tick(refreshInterval, func(t time.Time) tea.Msg { return tickMsg(t) })
}

type model struct {
	agg           *aggregator.Aggregator
	target        string
	sortBy        aggregator.SortField
	filter        string
	editing       bool
	helpOverlay   bool
	detailOverlay bool
	cursor        int
	width         int
	height        int
	started       time.Time
}

func (m model) Init() tea.Cmd { return tick() }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tickMsg:
		return m, tick()
	case tea.KeyMsg:
		if m.editing {
			return m.handleFilterKey(msg)
		}
		return m.handleKey(msg)
	}
	return m, nil
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// any key closes the help overlay
	if m.helpOverlay {
		m.helpOverlay = false
		return m, nil
	}
	// any key closes the detail overlay
	if m.detailOverlay {
		m.detailOverlay = false
		return m, nil
	}
	switch msg.String() {
	case "q", "Q", "ctrl+c":
		return m, tea.Quit
	case "?":
		m.helpOverlay = true
	case "/":
		m.editing = true
		m.filter = ""
	case "esc":
		m.filter = ""
		m.cursor = 0
	case "c":
		m.sortBy = aggregator.SortByCount
	case "t":
		m.sortBy = aggregator.SortByTotal
	case "a":
		m.sortBy = aggregator.SortByAvg
	case "x":
		m.sortBy = aggregator.SortByMax
	case "e":
		m.sortBy = aggregator.SortByErrors
	case "n":
		m.sortBy = aggregator.SortByName
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		m.cursor++ // clamped in View() after slice is built
	case "d", "D":
		m.detailOverlay = true
	}
	return m, nil
}

func (m model) handleFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEscape, tea.KeyEnter:
		m.editing = false
	case tea.KeyBackspace:
		if len(m.filter) > 0 {
			m.filter = m.filter[:len(m.filter)-1]
		}
	case tea.KeyRunes:
		m.filter += string(msg.Runes)
	}
	return m, nil
}

// ── View ──────────────────────────────────────────────────────────────────────

func (m model) View() string {
	if m.width == 0 {
		return "Initialising stracectl…"
	}
	if m.helpOverlay {
		return m.renderHelp()
	}
	if m.detailOverlay {
		return m.renderDetail()
	}

	w := m.width
	cw := colWidths(w)

	elapsed := time.Since(m.started).Round(time.Second)
	titleText := fmt.Sprintf(" stracectl  %-30s elapsed: %s ", m.target, elapsed)
	titleLine := titleStyle.Width(w).Render(titleText)

	statsLine := m.renderStatsBar(w)
	catLine := m.renderCategoryBar(w)
	summaryLine := m.renderSummary(w)

	div := divStyle.Render(strings.Repeat("─", w))
	hdr := renderHeader(cw, m.sortBy)

	var footer string
	if m.editing {
		footer = filterStyle.Render(fmt.Sprintf(" filter: %s█", m.filter))
	} else {
		hint := " q:quit  c:req▼  t:total  a:avg  x:max  e:errors  n:name  /:filter  ↑↓/jk:move  d:details  ?:help  esc:clear"
		if m.filter != "" {
			hint += fmt.Sprintf("   [filter: %q]", m.filter)
		}
		footer = footerStyle.Render(hint)
	}

	// anomaly alerts
	alerts := m.renderAlerts()

	// fixed UI lines: title, stats, cat, summary, div, hdr, div, [alerts?], div, footer
	fixedLines := 8
	if alerts != "" {
		fixedLines += strings.Count(alerts, "\n") + 1
	}
	maxRows := m.height - fixedLines
	if maxRows < 1 {
		maxRows = 1
	}

	stats := m.agg.Sorted(m.sortBy)
	if m.filter != "" {
		needle := strings.ToLower(m.filter)
		filtered := stats[:0]
		for _, s := range stats {
			if strings.Contains(s.Name, needle) {
				filtered = append(filtered, s)
			}
		}
		stats = filtered
	}

	// compute max count for sparkbar scaling
	var maxCount int64
	for _, s := range stats {
		if s.Count > maxCount {
			maxCount = s.Count
		}
	}

	// clamp cursor
	if m.cursor >= len(stats) {
		m.cursor = len(stats) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}

	if len(stats) > maxRows {
		stats = stats[:maxRows]
	}

	var sb strings.Builder
	sb.WriteString(titleLine + "\n")
	sb.WriteString(statsLine + "\n")
	sb.WriteString(catLine + "\n")
	sb.WriteString(summaryLine + "\n")
	sb.WriteString(div + "\n")
	sb.WriteString(hdr + "\n")
	sb.WriteString(div + "\n")
	if alerts != "" {
		sb.WriteString(alerts + "\n")
		sb.WriteString(div + "\n")
	}

	for i, s := range stats {
		errPctStr := "—"
		if s.Errors > 0 {
			errPctStr = fmt.Sprintf("%.0f%%", s.ErrPct())
		}

		bar := sparkBar(s.Count, maxCount, cw.bar)

		catTag := catStyle(s.Category).Render(fmt.Sprintf("%-5s", s.Category.String()))

		cursor := "  "
		if i == m.cursor {
			cursor = "► "
		}

		row := cursor + padR(s.Name, cw.name-2) +
			catTag +
			padL(formatCount(s.Count), cw.count) +
			" " + barFillStyle.Render(bar) + " " +
			padL(formatDur(s.AvgTime()), cw.avg) +
			padL(formatDur(s.MaxTime), cw.max) +
			padL(formatDur(s.TotalTime), cw.total) +
			padL(errPctStr, cw.errpct)

		var style lipgloss.Style
		if i == m.cursor {
			style = selectedRowStyle
		} else if s.ErrPct() >= hotErrPct {
			style = hotRowStyle
		} else if s.Errors > 0 {
			style = errRowStyle
		} else if s.AvgTime() >= slowAvgThreshold {
			style = slowRowStyle
		} else {
			style = rowStyle
		}

		sb.WriteString(style.Render(row) + "\n")
	}
	for i := len(stats); i < maxRows; i++ {
		sb.WriteString("\n")
	}

	sb.WriteString(div + "\n")
	sb.WriteString(footer)

	return sb.String()
}

// ── Stats bar ─────────────────────────────────────────────────────────────────

func (m model) renderStatsBar(w int) string {
	total := m.agg.Total()
	errs := m.agg.Errors()
	rate := m.agg.Rate()
	unique := m.agg.UniqueCount()

	errPct := 0.0
	if total > 0 {
		errPct = float64(errs) / float64(total) * 100
	}

	var errPart string
	if errs > 0 {
		errPart = fmt.Sprintf("errors: %s (%.1f%%)  ", formatCount(errs), errPct)
	} else {
		errPart = "errors: 0  "
	}

	text := fmt.Sprintf("  syscalls: %-8s  rate: %5.0f/s  unique: %-4d  %s",
		formatCount(total), rate, unique, errPart)
	return statsStyle.Width(w).Render(text)
}

// ── Category bar ──────────────────────────────────────────────────────────────

func (m model) renderCategoryBar(w int) string {
	bd := m.agg.CategoryBreakdown()
	total := m.agg.Total()

	order := []aggregator.Category{
		aggregator.CatIO,
		aggregator.CatFS,
		aggregator.CatNet,
		aggregator.CatMem,
		aggregator.CatProcess,
		aggregator.CatSignal,
		aggregator.CatOther,
	}

	var parts []string
	for _, cat := range order {
		cs, ok := bd[cat]
		if !ok || cs.Count == 0 {
			continue
		}
		pct := float64(cs.Count) / float64(total) * 100
		label := fmt.Sprintf("%s:%.0f%%", cat.String(), pct)
		parts = append(parts, catStyle(cat).Render(label))
	}

	line := "  " + strings.Join(parts, divStyle.Render("  │  "))
	return statsStyle.Width(w).Render(line)
}

// ── Plain-language summary ────────────────────────────────────────────────────

func (m model) renderSummary(w int) string {
	bd := m.agg.CategoryBreakdown()
	total := m.agg.Total()
	if total == 0 {
		return statsStyle.Width(w).Render("  Waiting for syscalls…")
	}

	// find the dominant category
	type kv struct {
		cat aggregator.Category
		pct float64
	}
	var sorted []kv
	for cat, cs := range bd {
		if cs.Count == 0 {
			continue
		}
		sorted = append(sorted, kv{cat, float64(cs.Count) / float64(total) * 100})
	}
	// simple selection sort for top-2 (small slice, no import needed)
	for i := 0; i < len(sorted)-1; i++ {
		max := i
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].pct > sorted[max].pct {
				max = j
			}
		}
		sorted[i], sorted[max] = sorted[max], sorted[i]
	}

	catSummary := map[aggregator.Category]string{
		aggregator.CatIO:      "reading and writing data",
		aggregator.CatFS:      "accessing the filesystem (stat, open, permissions)",
		aggregator.CatNet:     "networking (connect, send, recv)",
		aggregator.CatMem:     "managing memory (mmap, mprotect)",
		aggregator.CatProcess: "managing processes and threads",
		aggregator.CatSignal:  "handling signals",
		aggregator.CatOther:   "miscellaneous kernel calls",
	}

	var desc string
	if len(sorted) > 0 {
		top := sorted[0]
		desc = fmt.Sprintf("Process is mainly %s (%.0f%%)", catSummary[top.cat], top.pct)
		if len(sorted) > 1 && sorted[1].pct >= 10 {
			desc += fmt.Sprintf(", then %s (%.0f%%)", catSummary[sorted[1].cat], sorted[1].pct)
		}
	}

	errPct := float64(m.agg.Errors()) / float64(total) * 100
	var health string
	switch {
	case errPct == 0:
		health = " — ✓ no errors"
	case errPct < 15:
		health = fmt.Sprintf(" — ✓ %.0f%% errors (likely normal)", errPct)
	case errPct < 40:
		health = fmt.Sprintf(" — ⚠ %.0f%% errors (worth investigating)", errPct)
	default:
		health = fmt.Sprintf(" — ✗ %.0f%% errors (high, check alerts above)", errPct)
	}

	return statsStyle.Width(w).Render("  " + desc + health)
}

// ── Anomaly alerts ────────────────────────────────────────────────────────────

func (m model) renderAlerts() string {
	stats := m.agg.Sorted(aggregator.SortByErrors)
	var lines []string
	for _, s := range stats {
		if s.ErrPct() >= hotErrPct {
			expl := alertExplanation(s.Name)
			msg := fmt.Sprintf(" ⚠  %s: %.0f%% error rate (%d/%d calls)",
				s.Name, s.ErrPct(), s.Errors, s.Count)
			if expl != "" {
				msg += " — " + expl
			}
			lines = append(lines, alertStyle.Render(msg))
		} else if s.AvgTime() >= slowAvgThreshold {
			lines = append(lines,
				slowRowStyle.Render(fmt.Sprintf(" ⚡  %s: slow avg %s (max %s) — kernel spending time in this call",
					s.Name, formatDur(s.AvgTime()), formatDur(s.MaxTime))))
		}
	}
	return strings.Join(lines, "\n")
}

// alertExplanation gives a human-readable reason for common high-error syscalls.
func alertExplanation(name string) string {
	switch name {
	case "ioctl":
		return "terminal control failed — process likely has no TTY (running under sudo or piped)"
	case "openat", "open":
		return "files not found — often normal (dynamic linker searches multiple paths)"
	case "access", "faccessat":
		return "optional files are missing — usually harmless (checking for config files)"
	case "connect":
		return "connection attempts failed — may be Happy Eyeballs (IPv4/IPv6 race) or no route"
	case "recvfrom", "recv", "recvmsg":
		return "EAGAIN on non-blocking socket — normal for async I/O, not a real error"
	case "sendto", "send", "sendmsg":
		return "send failed — peer may have closed the connection"
	case "madvise":
		return "memory hint rejected by kernel — informational, not a real failure"
	case "prctl":
		return "process control rejected — may lack capabilities (seccomp or container policy)"
	case "statfs", "fstatfs":
		return "filesystem stat failed — path may be on a special fs (proc, tmpfs)"
	case "unlink", "unlinkat":
		return "tried to delete a non-existent file — may be cleanup of temp files"
	case "mkdir", "mkdirat":
		return "directory already exists — common during first-run initialisation"
	default:
		return ""
	}
}

// ── Detail overlay ───────────────────────────────────────────────────────────

func (m model) renderDetail() string {
	w := m.width
	if w == 0 {
		w = 80
	}

	stats := m.agg.Sorted(m.sortBy)
	if m.filter != "" {
		needle := strings.ToLower(m.filter)
		filtered := stats[:0]
		for _, s := range stats {
			if strings.Contains(s.Name, needle) {
				filtered = append(filtered, s)
			}
		}
		stats = filtered
	}

	if len(stats) == 0 {
		return detailDimStyle.Render("  no syscall selected")
	}
	idx := m.cursor
	if idx >= len(stats) {
		idx = len(stats) - 1
	}
	s := stats[idx]

	div := divStyle.Render(strings.Repeat("─", w))

	var sb strings.Builder
	titleText := fmt.Sprintf(" stracectl  details: %s  (press any key to close) ", s.Name)
	sb.WriteString(detailTitleStyle.Width(w).Render(titleText) + "\n")
	sb.WriteString(div + "\n")

	info := syscallInfo(s.Name)

	field := func(label, value string) {
		l := detailLabelStyle.Render(fmt.Sprintf("  %-18s", label))
		v := detailValueStyle.Render(value)
		sb.WriteString(l + v + "\n")
	}
	dimField := func(label, value string) {
		l := detailLabelStyle.Render(fmt.Sprintf("  %-18s", label))
		v := detailDimStyle.Render(value)
		sb.WriteString(l + v + "\n")
	}
	codeField := func(label, value string) {
		l := detailLabelStyle.Render(fmt.Sprintf("  %-18s", label))
		v := detailCodeStyle.Render(value)
		sb.WriteString(l + v + "\n")
	}
	section := func(title string) {
		sb.WriteString("\n")
		sb.WriteString(headerStyle.Render(" "+title) + "\n")
		sb.WriteString(div + "\n")
	}

	section("SYSCALL REFERENCE")
	field("Name", s.Name)
	field("Category", catStyle(s.Category).Render(s.Category.String()))
	field("Description", info.description)

	if info.signature != "" {
		codeField("Signature", info.signature)
	}

	if len(info.args) > 0 {
		section("ARGUMENTS")
		for _, a := range info.args {
			dimField(a[0], a[1])
		}
	}

	if info.returnValue != "" {
		section("RETURN VALUE")
		field("On success", info.returnValue)
		if info.errorHint != "" {
			field("On error", "-1, errno set")
			field("Common errors", info.errorHint)
		}
	}

	if info.notes != "" {
		section("NOTES")
		// word-wrap notes to terminal width - 22 (label indent)
		wrapWidth := w - 22
		if wrapWidth < 40 {
			wrapWidth = 40
		}
		for _, line := range wordWrap(info.notes, wrapWidth) {
			sb.WriteString(detailValueStyle.Render("  "+strings.Repeat(" ", 18)+line) + "\n")
		}
	}

	section("LIVE STATISTICS")
	field("Calls", formatCount(s.Count))
	field("Errors", fmt.Sprintf("%s  (%.0f%%)", formatCount(s.Errors), s.ErrPct()))
	field("Avg latency", formatDur(s.AvgTime()))
	field("Max latency", formatDur(s.MaxTime))
	field("Min latency", formatDur(s.MinTime))
	field("Total time", formatDur(s.TotalTime))

	if expl := alertExplanation(s.Name); expl != "" && s.ErrPct() >= hotErrPct {
		section("ANOMALY EXPLANATION")
		for _, line := range wordWrap(expl, w-22) {
			sb.WriteString(alertStyle.Render("  ⚠  "+line) + "\n")
		}
	}

	sb.WriteString("\n")
	sb.WriteString(div + "\n")
	sb.WriteString(footerStyle.Render(" press any key to return  │  ↑↓/jk to move between syscalls "))
	return sb.String()
}

// syscallDetail holds reference information for one syscall.
type syscallDetail struct {
	description string
	signature   string
	args        [][2]string // [name, description]
	returnValue string
	errorHint   string
	notes       string
}

// syscallInfo returns human-readable reference data for well-known syscalls.
// Unknown syscalls get a generic entry.
func syscallInfo(name string) syscallDetail {
	switch name {
	case "read":
		return syscallDetail{
			description: "Read bytes from a file descriptor into a buffer.",
			signature:   "read(fd, buf, count) → bytes_read",
			args:        [][2]string{{"fd", "open file descriptor to read from"}, {"buf", "destination buffer in user space"}, {"count", "maximum number of bytes to read"}},
			returnValue: "number of bytes read (0 = EOF)",
			errorHint:   "EAGAIN (non-blocking, no data), EBADF (bad fd), EFAULT (bad buffer), EINTR (signal)",
			notes:       "High read() counts often indicate heavy file or socket data transfer. EAGAIN is expected for non-blocking I/O and not a real error.",
		}
	case "write":
		return syscallDetail{
			description: "Write bytes from a buffer to a file descriptor.",
			signature:   "write(fd, buf, count) → bytes_written",
			args:        [][2]string{{"fd", "open file descriptor to write to"}, {"buf", "source buffer in user space"}, {"count", "number of bytes to write"}},
			returnValue: "number of bytes written",
			errorHint:   "EAGAIN (non-blocking, buffer full), EBADF, EPIPE (peer closed), EFAULT",
			notes:       "A short write (return < count) can happen on sockets or pipes; callers should loop.",
		}
	case "openat", "open":
		return syscallDetail{
			description: "Open or create a file, returning a file descriptor.",
			signature:   "openat(dirfd, pathname, flags, mode) → fd",
			args:        [][2]string{{"dirfd", "AT_FDCWD or directory fd for relative path"}, {"pathname", "path to file"}, {"flags", "O_RDONLY, O_WRONLY, O_CREAT, O_TRUNC, …"}, {"mode", "permission bits when O_CREAT is used"}},
			returnValue: "new file descriptor (≥ 0)",
			errorHint:   "ENOENT (not found), EACCES (permission), EMFILE (too many open fds), EEXIST (O_CREAT|O_EXCL)",
			notes:       "High ENOENT error rates are normal: the dynamic linker probes many paths when loading shared libraries.",
		}
	case "close":
		return syscallDetail{
			description: "Close a file descriptor, releasing the kernel resource.",
			signature:   "close(fd) → 0",
			args:        [][2]string{{"fd", "file descriptor to close"}},
			returnValue: "0",
			errorHint:   "EBADF (fd not open), EIO (deferred write-back failed — data may be lost)",
			notes:       "Never ignore EIO on close() — it means data was not written to disk.",
		}
	case "mmap", "mmap2":
		return syscallDetail{
			description: "Map files or devices into memory, or allocate anonymous memory.",
			signature:   "mmap(addr, length, prot, flags, fd, offset) → addr",
			args:        [][2]string{{"addr", "hint for mapping address (usually 0 = kernel decides)"}, {"length", "size in bytes"}, {"prot", "PROT_READ | PROT_WRITE | PROT_EXEC"}, {"flags", "MAP_PRIVATE, MAP_SHARED, MAP_ANONYMOUS, …"}, {"fd", "file to map, or -1 for anonymous"}, {"offset", "file offset (must be page-aligned)"}},
			returnValue: "virtual address of the mapping (hex)",
			errorHint:   "ENOMEM (out of virtual address space), EACCES, EINVAL",
			notes:       "MAP_ANONYMOUS|MAP_PRIVATE is the usual way malloc allocates large blocks from the kernel.",
		}
	case "munmap":
		return syscallDetail{
			description: "Remove a memory mapping created by mmap.",
			signature:   "munmap(addr, length) → 0",
			args:        [][2]string{{"addr", "start of the mapping (must be page-aligned)"}, {"length", "size to unmap"}},
			returnValue: "0",
			errorHint:   "EINVAL (addr not aligned or not mapped)",
		}
	case "mprotect":
		return syscallDetail{
			description: "Change memory protection attributes on a mapped region.",
			signature:   "mprotect(addr, len, prot) → 0",
			args:        [][2]string{{"addr", "page-aligned start address"}, {"len", "length in bytes"}, {"prot", "PROT_NONE | PROT_READ | PROT_WRITE | PROT_EXEC"}},
			returnValue: "0",
			errorHint:   "EACCES (e.g. trying to make a file-backed mapping writable without write permission), EINVAL",
			notes:       "Frequently called by the dynamic linker (ld.so) during library loading — setting sections read-only after relocation.",
		}
	case "madvise":
		return syscallDetail{
			description: "Give the kernel hints on expected memory usage patterns.",
			signature:   "madvise(addr, length, advice) → 0",
			args:        [][2]string{{"addr", "page-aligned start"}, {"length", "region size"}, {"advice", "MADV_NORMAL, MADV_SEQUENTIAL, MADV_DONTNEED, MADV_FREE, …"}},
			returnValue: "0",
			errorHint:   "EINVAL (unknown advice or bad addr/length), EACCES",
			notes:       "EACCES or EINVAL errors are informational — the kernel ignores hints it cannot honour. Not a real failure.",
		}
	case "brk":
		return syscallDetail{
			description: "Adjust the end of the data segment (heap boundary).",
			signature:   "brk(addr) → new_brk",
			args:        [][2]string{{"addr", "new end of heap (0 = query current value)"}},
			returnValue: "current break address",
			notes:       "Modern malloc implementations prefer mmap for large allocations; brk is used for the initial heap.",
		}
	case "fstat", "stat", "lstat", "newfstatat", "statx":
		return syscallDetail{
			description: "Retrieve file metadata (size, permissions, timestamps, inode).",
			signature:   "fstat(fd, statbuf) → 0  /  stat(pathname, statbuf) → 0",
			args:        [][2]string{{"fd / pathname", "file descriptor or path to inspect"}, {"statbuf", "struct stat to fill in"}},
			returnValue: "0",
			errorHint:   "ENOENT (path not found), EACCES, EBADF",
			notes:       "High fstat/stat rates are normal when an application polls file state or a web server serves many files.",
		}
	case "getdents", "getdents64":
		return syscallDetail{
			description: "Read directory entries from an open directory file descriptor.",
			signature:   "getdents64(fd, dirp, count) → bytes_read",
			args:        [][2]string{{"fd", "directory fd"}, {"dirp", "buffer for linux_dirent64 structs"}, {"count", "buffer size"}},
			returnValue: "bytes read (0 = end of directory)",
			errorHint:   "EBADF, ENOTDIR (fd is not a directory)",
			notes:       "Used by readdir(3). High counts suggest directory scanning (e.g. file watchers, recursive search).",
		}
	case "access", "faccessat", "faccessat2":
		return syscallDetail{
			description: "Check whether the calling process can access a file.",
			signature:   "access(pathname, mode) → 0",
			args:        [][2]string{{"pathname", "path to check"}, {"mode", "F_OK (exists?), R_OK, W_OK, X_OK"}},
			returnValue: "0 if access allowed",
			errorHint:   "ENOENT (not found — usually harmless), EACCES (permission denied)",
			notes:       "High ENOENT rates are expected: programs probe for optional config files. Not a real error.",
		}
	case "connect":
		return syscallDetail{
			description: "Initiate a connection on a socket.",
			signature:   "connect(sockfd, addr, addrlen) → 0",
			args:        [][2]string{{"sockfd", "open socket fd"}, {"addr", "target address (sockaddr_in, sockaddr_un, …)"}, {"addrlen", "sizeof(*addr)"}},
			returnValue: "0 on success",
			errorHint:   "ECONNREFUSED (port closed), ETIMEDOUT, ENETUNREACH, EINPROGRESS (non-blocking)",
			notes:       "Errors are common with Happy Eyeballs (RFC 8305): both IPv4 and IPv6 are tried in parallel; the loser always fails with ECONNREFUSED or ETIMEDOUT.",
		}
	case "accept", "accept4":
		return syscallDetail{
			description: "Accept a new incoming connection on a listening socket.",
			signature:   "accept4(sockfd, addr, addrlen, flags) → fd",
			args:        [][2]string{{"sockfd", "listening socket fd"}, {"addr", "filled with peer address"}, {"flags", "SOCK_NONBLOCK, SOCK_CLOEXEC"}},
			returnValue: "new connected socket fd",
			errorHint:   "EAGAIN (no pending connections, non-blocking), EMFILE (too many open fds)",
		}
	case "recvfrom", "recv", "recvmsg", "recvmmsg":
		return syscallDetail{
			description: "Receive data from a socket.",
			signature:   "recvfrom(sockfd, buf, len, flags, src_addr, addrlen) → bytes",
			args:        [][2]string{{"sockfd", "connected or unconnected socket"}, {"buf", "receive buffer"}, {"flags", "MSG_DONTWAIT, MSG_PEEK, MSG_WAITALL, …"}},
			returnValue: "bytes received (0 = peer closed)",
			errorHint:   "EAGAIN/EWOULDBLOCK (non-blocking, no data yet — normal), ECONNRESET",
			notes:       "EAGAIN on a non-blocking socket is not a real error — the event loop will retry.",
		}
	case "sendto", "send", "sendmsg", "sendmmsg":
		return syscallDetail{
			description: "Send data through a socket.",
			signature:   "sendto(sockfd, buf, len, flags, dest_addr, addrlen) → bytes",
			args:        [][2]string{{"sockfd", "socket fd"}, {"buf", "data to send"}, {"flags", "MSG_DONTWAIT, MSG_NOSIGNAL, …"}},
			returnValue: "bytes sent",
			errorHint:   "EPIPE (peer closed — usually triggers SIGPIPE too), EAGAIN, ECONNRESET",
		}
	case "epoll_wait", "epoll_pwait":
		return syscallDetail{
			description: "Wait for events on an epoll file descriptor.",
			signature:   "epoll_wait(epfd, events, maxevents, timeout) → n_events",
			args:        [][2]string{{"epfd", "epoll instance fd"}, {"events", "array of epoll_event to fill"}, {"maxevents", "max events to return"}, {"timeout", "ms to wait (-1 = block forever)"}},
			returnValue: "number of ready fds (0 = timeout)",
			notes:       "The main blocking call in event-driven servers (nginx, Node.js, Go net poller). High count = many I/O events.",
		}
	case "epoll_ctl":
		return syscallDetail{
			description: "Add, modify, or remove a file descriptor from an epoll instance.",
			signature:   "epoll_ctl(epfd, op, fd, event) → 0",
			args:        [][2]string{{"op", "EPOLL_CTL_ADD, EPOLL_CTL_MOD, EPOLL_CTL_DEL"}, {"fd", "target file descriptor"}, {"event", "epoll_event with events mask and user data"}},
			returnValue: "0",
			errorHint:   "ENOENT (DEL/MOD on fd not in epoll), EEXIST (ADD on already registered fd)",
		}
	case "poll", "ppoll":
		return syscallDetail{
			description: "Wait for events on a set of file descriptors.",
			signature:   "poll(fds, nfds, timeout) → n_ready",
			args:        [][2]string{{"fds", "array of pollfd structs"}, {"nfds", "number of fds"}, {"timeout", "milliseconds (-1 = block)"}},
			returnValue: "number of fds with events (0 = timeout, -1 = error)",
		}
	case "futex":
		return syscallDetail{
			description: "Fast user-space locking primitive — the kernel backing for mutexes and condition variables.",
			signature:   "futex(uaddr, op, val, timeout, uaddr2, val3) → 0 or value",
			args:        [][2]string{{"uaddr", "address of the futex word (shared between threads)"}, {"op", "FUTEX_WAIT, FUTEX_WAKE, FUTEX_LOCK_PI, …"}, {"val", "expected value (for WAIT) or wake count (for WAKE)"}},
			notes:       "Most of the time futex stays in user space (no syscall). A syscall happens only when a thread must actually sleep or be woken. High counts suggest heavy lock contention.",
		}
	case "clone", "clone3":
		return syscallDetail{
			description: "Create a new process or thread, with fine-grained control over shared resources.",
			signature:   "clone(flags, stack, ptid, ctid, regs) → child_pid",
			args:        [][2]string{{"flags", "CLONE_THREAD, CLONE_VM, CLONE_FS, SIGCHLD, … (dozens of flags)"}, {"stack", "new stack pointer for the child (0 = copy)"}},
			returnValue: "child PID in parent, 0 in child",
			notes:       "pthread_create(3) uses clone with CLONE_THREAD|CLONE_VM. fork(2) uses clone with SIGCHLD only.",
		}
	case "execve", "execveat":
		return syscallDetail{
			description: "Replace the current process image with a new program.",
			signature:   "execve(pathname, argv, envp) → (does not return on success)",
			args:        [][2]string{{"pathname", "path to executable"}, {"argv", "argument vector (NULL-terminated array)"}, {"envp", "environment strings"}},
			returnValue: "does not return; -1 on error",
			errorHint:   "ENOENT (not found), EACCES (not executable), ENOEXEC (bad ELF), ENOMEM",
		}
	case "exit", "exit_group":
		return syscallDetail{
			description: "Terminate the calling thread (exit) or all threads in the thread group (exit_group).",
			signature:   "exit_group(status) → (does not return)",
			args:        [][2]string{{"status", "exit code (low 8 bits visible to waitpid)"}},
			notes:       "exit_group is what libc exit(3) calls. glibc calls exit_group so all threads terminate cleanly.",
		}
	case "wait4", "waitpid", "waitid":
		return syscallDetail{
			description: "Wait for a child process to change state.",
			signature:   "wait4(pid, status, options, rusage) → child_pid",
			args:        [][2]string{{"pid", "-1 = any child, >0 = specific PID"}, {"options", "WNOHANG (non-blocking), WUNTRACED, WCONTINUED"}},
			returnValue: "PID of child that changed state (0 with WNOHANG if none ready)",
		}
	case "ioctl":
		return syscallDetail{
			description: "Device-specific control operations on a file descriptor.",
			signature:   "ioctl(fd, request, argp) → 0 or value",
			args:        [][2]string{{"fd", "open file descriptor (device, socket, terminal, …)"}, {"request", "device-specific command code (TIOCGWINSZ, FIONREAD, …)"}, {"argp", "pointer to in/out argument"}},
			returnValue: "0 or a request-specific value",
			errorHint:   "ENOTTY (fd is not a terminal — very common when stdout is piped), EINVAL, ENODEV",
			notes:       "ENOTTY is expected when the process checks for a TTY but is running under sudo, in a container, or with piped output. Not a real failure.",
		}
	case "prctl":
		return syscallDetail{
			description: "Control various process attributes (name, seccomp, capabilities, …).",
			signature:   "prctl(option, arg2, arg3, arg4, arg5) → 0 or value",
			args:        [][2]string{{"option", "PR_SET_NAME, PR_SET_SECCOMP, PR_CAP_AMBIENT, PR_SET_DUMPABLE, …"}},
			returnValue: "0 or option-specific value",
			errorHint:   "EPERM (capability required), EINVAL (unknown option)",
			notes:       "EPERM on prctl is common in containers with restricted capabilities or seccomp profiles.",
		}
	case "rt_sigaction", "sigaction":
		return syscallDetail{
			description: "Install or query a signal handler.",
			signature:   "rt_sigaction(signum, act, oldact, sigsetsize) → 0",
			args:        [][2]string{{"signum", "signal number (SIGINT, SIGSEGV, …)"}, {"act", "new sigaction struct (NULL = query only)"}, {"oldact", "previous handler (NULL = discard)"}},
			returnValue: "0",
		}
	case "rt_sigprocmask", "sigprocmask":
		return syscallDetail{
			description: "Block, unblock, or query the set of blocked signals.",
			signature:   "rt_sigprocmask(how, set, oldset, sigsetsize) → 0",
			args:        [][2]string{{"how", "SIG_BLOCK, SIG_UNBLOCK, SIG_SETMASK"}, {"set", "new signal mask (NULL = query)"}, {"oldset", "previous mask"}},
			notes:       "Called very frequently by Go and pthreads runtimes around goroutine/thread switches.",
		}
	case "getpid":
		return syscallDetail{
			description: "Return the process ID of the calling process.",
			signature:   "getpid() → pid",
			notes:       "Modern Linux caches the PID in the vDSO — this syscall may never actually enter the kernel.",
		}
	case "getuid", "geteuid", "getgid", "getegid":
		return syscallDetail{
			description: "Return the real/effective user or group ID of the calling process.",
			signature:   "getuid() → uid",
			notes:       "Very cheap; usually cached by libc. Frequent calls suggest credential-checking code paths.",
		}
	case "lseek", "llseek":
		return syscallDetail{
			description: "Reposition the read/write offset of a file descriptor.",
			signature:   "lseek(fd, offset, whence) → new_offset",
			args:        [][2]string{{"whence", "SEEK_SET (absolute), SEEK_CUR (relative), SEEK_END (from end)"}},
			returnValue: "resulting file offset",
			errorHint:   "ESPIPE (fd is a pipe or socket — not seekable), EINVAL",
		}
	case "pipe", "pipe2":
		return syscallDetail{
			description: "Create a unidirectional data channel (pipe) between two file descriptors.",
			signature:   "pipe2(pipefd[2], flags) → 0",
			args:        [][2]string{{"pipefd", "[0]=read end, [1]=write end"}, {"flags", "O_CLOEXEC, O_NONBLOCK, O_DIRECT"}},
			returnValue: "0",
		}
	case "dup", "dup2", "dup3":
		return syscallDetail{
			description: "Duplicate a file descriptor.",
			signature:   "dup2(oldfd, newfd) → newfd",
			args:        [][2]string{{"oldfd", "fd to duplicate"}, {"newfd", "desired fd number (closed first if open)"}},
			returnValue: "new file descriptor",
		}
	case "socket":
		return syscallDetail{
			description: "Create a communication endpoint (socket).",
			signature:   "socket(domain, type, protocol) → fd",
			args:        [][2]string{{"domain", "AF_INET, AF_INET6, AF_UNIX, AF_NETLINK, …"}, {"type", "SOCK_STREAM, SOCK_DGRAM, SOCK_RAW | SOCK_NONBLOCK | SOCK_CLOEXEC"}, {"protocol", "0 (auto), IPPROTO_TCP, IPPROTO_UDP, …"}},
			returnValue: "new socket fd",
		}
	case "bind":
		return syscallDetail{
			description: "Assign a local address to a socket.",
			signature:   "bind(sockfd, addr, addrlen) → 0",
			args:        [][2]string{{"addr", "local address to bind (port + IP or Unix path)"}},
			errorHint:   "EADDRINUSE (port already in use), EACCES (port < 1024 without CAP_NET_BIND_SERVICE)",
		}
	case "listen":
		return syscallDetail{
			description: "Mark a socket as passive (ready to accept connections).",
			signature:   "listen(sockfd, backlog) → 0",
			args:        [][2]string{{"backlog", "max length of pending connection queue"}},
		}
	case "setsockopt", "getsockopt":
		return syscallDetail{
			description: "Set or get socket options (timeouts, buffers, TCP_NODELAY, SO_REUSEADDR, …).",
			signature:   "setsockopt(sockfd, level, optname, optval, optlen) → 0",
			args:        [][2]string{{"level", "SOL_SOCKET, IPPROTO_TCP, IPPROTO_IP, …"}, {"optname", "SO_REUSEADDR, SO_KEEPALIVE, TCP_NODELAY, SO_RCVBUF, …"}},
		}
	case "getsockname", "getpeername":
		return syscallDetail{
			description: "Get the local (getsockname) or remote (getpeername) address of a socket.",
			signature:   "getsockname(sockfd, addr, addrlen) → 0",
		}
	case "getrandom":
		return syscallDetail{
			description: "Obtain cryptographically secure random bytes from the kernel.",
			signature:   "getrandom(buf, buflen, flags) → bytes_filled",
			args:        [][2]string{{"flags", "0 (block until entropy ready), GRND_NONBLOCK, GRND_RANDOM"}},
			notes:       "Preferred over /dev/urandom. Called at startup by TLS libraries and language runtimes for seed material.",
		}
	case "statfs", "fstatfs":
		return syscallDetail{
			description: "Get filesystem statistics (type, free space, block size, …).",
			signature:   "statfs(pathname, buf) → 0",
			args:        [][2]string{{"pathname", "path on the filesystem to inspect"}, {"buf", "struct statfs to fill"}},
			errorHint:   "ENOENT, EACCES, ENOSYS (on special filesystems like /proc)",
			notes:       "Errors on /proc or /sys are expected — those filesystems may not support statfs.",
		}
	case "fcntl":
		return syscallDetail{
			description: "Perform miscellaneous operations on a file descriptor (flags, locks, async I/O).",
			signature:   "fcntl(fd, cmd, arg) → value",
			args:        [][2]string{{"cmd", "F_GETFL, F_SETFL (O_NONBLOCK), F_GETFD, F_SETFD (FD_CLOEXEC), F_DUPFD, F_SETLK, …"}},
		}
	case "sendfile", "copy_file_range":
		return syscallDetail{
			description: "Transfer data between two file descriptors entirely in kernel space.",
			signature:   "sendfile(out_fd, in_fd, offset, count) → bytes_sent",
			notes:       "Zero-copy: data never crosses user space. Used by web servers to send file contents over sockets.",
		}
	case "prlimit64":
		return syscallDetail{
			description: "Get or set resource limits (CPU, memory, open files, …) for a process.",
			signature:   "prlimit64(pid, resource, new_limit, old_limit) → 0",
			args:        [][2]string{{"resource", "RLIMIT_NOFILE, RLIMIT_AS, RLIMIT_STACK, RLIMIT_CORE, …"}, {"pid", "0 = calling process"}},
		}
	case "eventfd", "eventfd2":
		return syscallDetail{
			description: "Create a file descriptor for event notification between threads/processes.",
			signature:   "eventfd2(initval, flags) → fd",
			args:        [][2]string{{"initval", "initial counter value"}, {"flags", "EFD_NONBLOCK, EFD_CLOEXEC, EFD_SEMAPHORE"}},
			notes:       "Used by Go runtime and libuv/libevent to wake up blocked pollers without a pipe.",
		}
	case "set_tid_address":
		return syscallDetail{
			description: "Set the address that the kernel will clear when the thread exits.",
			signature:   "set_tid_address(tidptr) → tid",
			notes:       "Called once at thread startup by glibc. Used for robust futex cleanup on thread exit.",
		}
	case "arch_prctl":
		return syscallDetail{
			description: "Set architecture-specific thread state (e.g. FS/GS segment base for TLS).",
			signature:   "arch_prctl(code, addr) → 0",
			args:        [][2]string{{"code", "ARCH_SET_FS (set FS base for thread-local storage), ARCH_GET_FS, …"}},
			notes:       "Called once per thread by glibc to initialise thread-local storage (TLS). Normal during startup.",
		}
	default:
		return syscallDetail{
			description: fmt.Sprintf("Kernel syscall %q — no reference entry available.", name),
			notes:       "See 'man 2 " + name + "' for full documentation.",
		}
	}
}

// wordWrap splits text into lines no longer than maxWidth characters.
func wordWrap(text string, maxWidth int) []string {
	if maxWidth <= 0 || len(text) <= maxWidth {
		return []string{text}
	}
	var lines []string
	words := strings.Fields(text)
	current := ""
	for _, w := range words {
		if current == "" {
			current = w
		} else if len(current)+1+len(w) <= maxWidth {
			current += " " + w
		} else {
			lines = append(lines, current)
			current = w
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}

// ── Help overlay ─────────────────────────────────────────────────────────────

func (m model) renderHelp() string {
	w := m.width
	if w == 0 {
		w = 80
	}

	var sb strings.Builder

	titleLine := titleStyle.Width(w).Render(" stracectl — help  (press any key to close) ")
	div := divStyle.Render(strings.Repeat("─", w))

	sb.WriteString(titleLine + "\n")
	sb.WriteString(div + "\n")

	section := func(title string) {
		sb.WriteString("\n")
		sb.WriteString(headerStyle.Render(" "+title) + "\n")
		sb.WriteString(div + "\n")
	}
	row := func(key, desc string) {
		sb.WriteString(activeSortStyle.Render(fmt.Sprintf("  %-12s", key)))
		sb.WriteString(rowStyle.Render(desc) + "\n")
	}

	section("COLUMNS")
	row("SYSCALL", "name of the kernel function called by the process")
	row("CAT", "category: I/O · FS · NET · MEM · PROC · SIG · OTHER")
	row("COUNT", "total number of times this syscall was called")
	row("FREQ", "bar showing count relative to the most-called syscall")
	row("AVG", "average time the kernel spent executing this syscall")
	row("MAX", "peak (worst) latency — outliers that avg hides")
	row("TOTAL", "cumulative CPU time spent inside this syscall")
	row("ERR%", "percentage of calls that returned an error")

	section("ROW COLOURS")
	row("white", "normal — no issues detected")
	row("yellow", "slow — AVG latency ≥ 5ms (kernel spending time here)")
	row("orange", "some errors, but ERR% < 50% (often harmless)")
	row("red", "critical — more than half of all calls are failing")

	section("CATEGORY BAR")
	row("I/O", "read, write, openat, close — file descriptor operations")
	row("FS", "stat, fstat, access, lseek — filesystem metadata")
	row("NET", "socket, connect, send, recv, epoll — networking")
	row("MEM", "mmap, munmap, mprotect, madvise — memory management")
	row("PROC", "clone, execve, wait, prctl — process/thread control")
	row("SIG", "rt_sigaction, sigprocmask — signal handling")

	section("COMMON PATTERNS")
	row("openat ERR%", "dynamic linker searches multiple paths — usually harmless")
	row("recvfrom ERR%", "EAGAIN on non-blocking socket — normal for async I/O")
	row("connect ERR%", "Happy Eyeballs: IPv4 and IPv6 tried in parallel, loser fails")
	row("ioctl ERR%", "process has no TTY (running under sudo or piped)")
	row("madvise ERR%", "memory hints rejected by kernel — informational")
	row("high I/O%", "process is doing heavy file or socket data transfer")
	row("high FS%", "process is scanning directories or checking many files")
	row("high SIG%", "many signal handlers registered — common during lib init")

	section("KEYBOARD SHORTCUTS")
	row("↑ / k", "move selection up")
	row("↓ / j", "move selection down")
	row("d / D", "open detail overlay for selected syscall")
	row("c", "sort by COUNT (most called first)")
	row("t", "sort by TOTAL time (most CPU in kernel)")
	row("a", "sort by AVG latency")
	row("x", "sort by MAX latency (find worst outlier)")
	row("e", "sort by error count")
	row("n", "sort alphabetically")
	row("/", "filter: type a syscall name to narrow the list")
	row("esc", "clear filter / deselect")
	row("?", "this help screen")
	row("q / Ctrl+C", "quit")

	sb.WriteString("\n")
	sb.WriteString(footerStyle.Render(" press any key to return "))

	return sb.String()
}

// ── Column layout ─────────────────────────────────────────────────────────────

type cols struct {
	name, cat, count, bar, avg, max, total, errpct int
}

func colWidths(w int) cols {
	cat, count, avg, max, total, errpct := 6, 9, 10, 10, 11, 7
	barW := 12
	name := w - cat - count - barW - 2 - avg - max - total - errpct
	if name < 14 {
		name = 14
	}
	return cols{name, cat, count, barW, avg, max, total, errpct}
}

func renderHeader(cw cols, sortBy aggregator.SortField) string {
	mark := func(f aggregator.SortField, label string) string {
		if sortBy == f {
			return activeSortStyle.Render(label + "▼")
		}
		return label + " "
	}
	return headerStyle.Render(
		padR("SYSCALL", cw.name) +
			padR("CAT", cw.cat) +
			padL(mark(aggregator.SortByCount, "REQ"), cw.count) +
			" " + padR("FREQ", cw.bar+1) +
			padL(mark(aggregator.SortByAvg, "AVG"), cw.avg) +
			padL(mark(aggregator.SortByMax, "MAX"), cw.max) +
			padL(mark(aggregator.SortByTotal, "TOTAL"), cw.total) +
			padL(mark(aggregator.SortByErrors, "ERR%"), cw.errpct),
	)
}

// ── Spark bar ─────────────────────────────────────────────────────────────────

func sparkBar(count, maxCount int64, width int) string {
	if maxCount == 0 || width <= 0 {
		return strings.Repeat("░", width)
	}
	filled := int(float64(count) / float64(maxCount) * float64(width))
	if filled > width {
		filled = width
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

// ── Category style helper ─────────────────────────────────────────────────────

func catStyle(c aggregator.Category) lipgloss.Style {
	switch c {
	case aggregator.CatIO:
		return catIOStyle
	case aggregator.CatFS:
		return catFSStyle
	case aggregator.CatNet:
		return catNetStyle
	case aggregator.CatMem:
		return catMemStyle
	case aggregator.CatProcess:
		return catProcStyle
	case aggregator.CatSignal:
		return catSigStyle
	default:
		return catOthStyle
	}
}

// ── Formatting helpers ────────────────────────────────────────────────────────

func padR(s string, n int) string {
	if len(s) >= n {
		return s[:n-1] + " "
	}
	return s + strings.Repeat(" ", n-len(s))
}

func padL(s string, n int) string {
	if len(s) >= n {
		return s[:n-1] + " "
	}
	return strings.Repeat(" ", n-len(s)) + s
}

func formatDur(d time.Duration) string {
	if d == 0 {
		return "—"
	}
	ns := d.Nanoseconds()
	switch {
	case ns < 1_000:
		return fmt.Sprintf("%dns", ns)
	case ns < 1_000_000:
		return fmt.Sprintf("%.1fµs", float64(ns)/1_000)
	case ns < 1_000_000_000:
		return fmt.Sprintf("%.1fms", float64(ns)/1_000_000)
	default:
		return fmt.Sprintf("%.2fs", float64(ns)/1_000_000_000)
	}
}

func formatCount(n int64) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

// ── Entry point ───────────────────────────────────────────────────────────────

// Run starts the full-screen TUI backed by agg.
func Run(agg *aggregator.Aggregator, target string) error {
	m := model{
		agg:     agg,
		target:  target,
		sortBy:  aggregator.SortByCount,
		started: time.Now(),
	}
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
