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

	barFillStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("63"))
	barEmptyStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("236"))

	divStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("238"))

	footerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("242"))

	filterStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("229"))

	alertStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("196"))
)

// ── Thresholds for visual anomaly detection ───────────────────────────────────

const (
	slowAvgThreshold = 5 * time.Millisecond // avg latency considered slow
	hotErrPct        = 50.0                 // % errors considered critical
)

// ── BubbleTea model ───────────────────────────────────────────────────────────

type tickMsg time.Time

func tick() tea.Cmd {
	return tea.Tick(refreshInterval, func(t time.Time) tea.Msg { return tickMsg(t) })
}

type model struct {
	agg         *aggregator.Aggregator
	target      string
	sortBy      aggregator.SortField
	filter      string
	editing     bool
	helpOverlay bool
	width       int
	height      int
	started     time.Time
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
		hint := " q:quit  c:req▼  t:total  a:avg  x:max  e:errors  n:name  /:filter  ?:help  esc:clear"
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

	for _, s := range stats {
		errPctStr := "—"
		if s.Errors > 0 {
			errPctStr = fmt.Sprintf("%.0f%%", s.ErrPct())
		}

		bar := sparkBar(s.Count, maxCount, cw.bar)

		catTag := catStyle(s.Category).Render(fmt.Sprintf("%-5s", s.Category.String()))

		row := padR(s.Name, cw.name) +
			catTag +
			padL(formatCount(s.Count), cw.count) +
			" " + barFillStyle.Render(bar) + " " +
			padL(formatDur(s.AvgTime()), cw.avg) +
			padL(formatDur(s.MaxTime), cw.max) +
			padL(formatDur(s.TotalTime), cw.total) +
			padL(errPctStr, cw.errpct)

		style := rowStyle
		if s.ErrPct() >= hotErrPct {
			style = hotRowStyle
		} else if s.Errors > 0 {
			style = errRowStyle
		} else if s.AvgTime() >= slowAvgThreshold {
			style = slowRowStyle
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
	row("c", "sort by COUNT (most called first)")
	row("t", "sort by TOTAL time (most CPU in kernel)")
	row("a", "sort by AVG latency")
	row("x", "sort by MAX latency (find worst outlier)")
	row("e", "sort by error count")
	row("n", "sort alphabetically")
	row("/", "filter: type a syscall name to narrow the list")
	row("esc", "clear filter")
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
