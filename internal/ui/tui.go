// Package ui provides the BubbleTea TUI for stracectl.
package ui

import (
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"

	"github.com/fabianoflorentino/stracectl/internal/aggregator"
)

const (
	refreshInterval  = 200 * time.Millisecond
	slowAvgThreshold = 5 * time.Millisecond // avg latency considered slow
	hotErrPct        = 50.0                 // % errors considered critical
)

// ── Styles ────────────────────────────────────────────────────────────────────
var (
	titleStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Background(lipgloss.Color("63")).Bold(true)
	statsStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("248")).Background(lipgloss.Color("235"))
	catIOStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("75"))
	catFSStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("149"))
	catNetStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	catMemStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("183"))
	catProcStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("210"))
	catSigStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	catOthStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	headerStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Bold(true)
	activeSortStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("118")).Bold(true)
	rowStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	errRowStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))            // row with >0 errors but error rate below the warning threshold
	hotRowStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true) // row with very high error rate (>= 50 %)
	slowRowStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("227")).Bold(true) // row whose avg latency exceeds the warning threshold
	barFillStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("63"))
	errNumStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	slowAvgStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("227"))
	divStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	footerStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("242"))
	filterStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("229"))
	alertStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	selectedRowStyle = lipgloss.NewStyle().Background(lipgloss.Color("237")).Foreground(lipgloss.Color("255")).Bold(true)
	detailTitleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Background(lipgloss.Color("25")).Bold(true)
	detailLabelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Bold(true)
	detailValueStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	detailDimStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	detailCodeStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("149"))
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
	logOverlay    bool
	filesOverlay  bool
	logOffset     int
	filesOffset   int
	processDone   bool
	cursor        int
	width         int
	height        int
	sizeMisses    int // number of consecutive ticks without a WindowSizeMsg
	started       time.Time
}

func (m model) Init() tea.Cmd { return tick() }

// processDeadMsg is sent to the program when the traced process exits.
// The TUI marks itself as done and shows a banner, but does NOT quit
// automatically — the user reviews the final data and presses q.
type processDeadMsg struct{}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case processDeadMsg:
		m.processDone = true
		return m, nil
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// record reception of a WindowSizeMsg for later diagnosis
		recordUIEvent("window-size", m.width, m.height)
	case tickMsg:
		// If we never received a WindowSizeMsg, the TUI would stay stuck
		// showing "Initialising stracectl…". After a few ticks, attempt a
		// fallback terminal-size detection so the UI can render in degraded
		// environments (sudo, re-exec, or piped stdout) instead of freezing.
		if m.width == 0 {
			m.sizeMisses++
			if m.sizeMisses >= 5 {
				w, h := detectFallbackSize()
				m.width = w
				m.height = h
				// record diagnostic hint for remote debugging
				recordFallbackEvent(w, h)
			}
		} else {
			m.sizeMisses = 0
		}
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
	// in the detail overlay: navigate or close
	if m.detailOverlay {
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			m.cursor++ // clamped in renderDetail
		case "q", "Q", "ctrl+c":
			return m, tea.Quit
		default:
			m.detailOverlay = false
		}
		return m, nil
	}
	// in the log overlay: scroll or close
	if m.logOverlay {
		switch msg.String() {
		case "up", "k":
			if m.logOffset > 0 {
				m.logOffset--
			}
		case "down", "j":
			m.logOffset++
		case "q", "Q", "ctrl+c":
			return m, tea.Quit
		default:
			m.logOverlay = false
		}
		return m, nil
	}
	// in the files overlay: scroll or close
	if m.filesOverlay {
		switch msg.String() {
		case "up", "k":
			if m.filesOffset > 0 {
				m.filesOffset--
			}
		case "down", "j":
			m.filesOffset++
		case "q", "Q", "ctrl+c":
			return m, tea.Quit
		default:
			m.filesOverlay = false
		}
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
	case "m":
		m.sortBy = aggregator.SortByMin
	case "x":
		m.sortBy = aggregator.SortByMax
	case "e":
		m.sortBy = aggregator.SortByErrors
	case "n":
		m.sortBy = aggregator.SortByName
	case "g":
		m.sortBy = aggregator.SortByCategory
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		m.cursor++ // clamped in View() after slice is built
	case "d", "D", "enter", " ":
		m.detailOverlay = true
	case "l", "L":
		m.logOverlay = true
		m.logOffset = -1 // -1 signals "scroll to bottom" on first render
	case "f", "F":
		m.filesOverlay = true
		m.filesOffset = 0
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
	if m.filesOverlay {
		return m.renderFiles()
	}
	if m.logOverlay {
		return m.renderLog()
	}

	w := m.width
	cw := colWidths(w)

	// Single header bar: title + target on the left, live stats on the right.
	total := m.agg.Total()
	errs := m.agg.Errors()
	rate := m.agg.Rate()
	unique := m.agg.UniqueCount()
	elapsed := time.Since(m.started).Round(time.Second)

	procLabel := m.target
	if pi := m.agg.GetProcInfo(); pi.Comm != "" {
		procLabel = fmt.Sprintf("%s[%d]", pi.Comm, pi.PID)
	}
	left := fmt.Sprintf(" stracectl  %s  +%s ", procLabel, elapsed)
	right := fmt.Sprintf(" syscalls: %s  rate: %.0f/s  errors: %s  unique: %d ",
		formatCount(total), rate, formatCount(errs), unique)
	gap := w - len(left) - len(right)
	if gap < 0 {
		gap = 0
	}
	titleLine := titleStyle.Render(left + strings.Repeat(" ", gap) + right)

	catLine := m.renderCategoryBar(w)

	div := divStyle.Render(strings.Repeat("─", w))
	hdr := renderHeader(cw, m.sortBy)

	shortcuts := " q:quit  c:calls▼  t:total  a:avg  m:min  x:max  e:errors  n:name  g:category  /:filter  ↑↓/jk:move  enter/d:details  l:log  f:files  ?:help  esc:clear"
	if m.filter != "" {
		shortcuts += fmt.Sprintf("   [filter: %q]", m.filter)
	}

	var footer string
	if m.editing {
		footer = filterStyle.Render(fmt.Sprintf(" filter: %s█", m.filter))
	} else if m.processDone {
		footer = footerStyle.Render(shortcuts)
	} else {
		footer = footerStyle.Render(shortcuts)
	}

	// anomaly alerts
	alerts := m.renderAlerts()

	// fixed UI lines: title+stats, div, cat, div, hdr, div, bottom-div, footer-sep-div + footer lines
	// baseline: 6 header lines + 1 end-of-rows div + 1 footer-sep div + footer height
	footerLines := strings.Count(footer, "\n") + 1
	fixedLines := 8 + footerLines
	if alerts != "" {
		// alert header line + alert content lines
		fixedLines += strings.Count(alerts, "\n") + 2
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

	// compute scroll offset so the selected row is always visible
	scrollOffset := 0
	if len(stats) > maxRows {
		if m.cursor >= maxRows {
			scrollOffset = m.cursor - maxRows + 1
		}
		stats = stats[scrollOffset : scrollOffset+maxRows]
	}

	var sb strings.Builder
	sb.WriteString(titleLine + "\n")
	sb.WriteString(div + "\n")
	sb.WriteString(catLine + "\n")
	sb.WriteString(div + "\n")
	sb.WriteString(hdr + "\n")
	sb.WriteString(div + "\n")

	for i, s := range stats {
		bar := sparkBar(s.Count, maxCount, cw.bar)

		// Prepare per-cell values with strict truncation to avoid overflow
		nameVal := truncateToWidth(sanitizeForTUI(s.Name), cw.name-2)

		// File column: show top-1 observed path for this syscall, truncated.
		fileVal := ""
		if len(s.Files) > 0 {
			fileVal = truncateToWidth(sanitizeForTUI(s.Files[0].Path), cw.file)
		}

		// Category tag: truncate visible label then apply style and pad to width.
		catLabel := truncateToWidth(s.Category.String(), cw.cat)
		catTag := catStyle(s.Category).Render(padR(catLabel, cw.cat))

		cursor := "  "
		if i+scrollOffset == m.cursor {
			cursor = "► "
		}

		// Per-cell colored parts: slow avg in yellow, errors in red.
		avgDur := s.AvgTime()
		var avgPart string
		if avgDur >= slowAvgThreshold {
			durStr := formatDur(avgDur)
			pad := max(0, cw.avg-lipgloss.Width(durStr))
			avgPart = strings.Repeat(" ", pad) + slowAvgStyle.Render(durStr)
		} else {
			avgPart = padL(formatDur(avgDur), cw.avg)
		}

		// Min time cell
		minDur := s.MinTime
		minPart := padL(formatDur(minDur), cw.min)

		var errCountPart, errPctPart string
		if s.Errors > 0 {
			cs := formatCount(s.Errors)
			ps := fmt.Sprintf("%.0f%%", s.ErrPct())
			errCountPart = strings.Repeat(" ", max(0, cw.errors-len(cs))) + errNumStyle.Render(cs)
			errPctPart = strings.Repeat(" ", max(0, cw.errpct-len(ps))) + errNumStyle.Render(ps)
		} else {
			errCountPart = padL("—", cw.errors)
			errPctPart = padL("—", cw.errpct)
		}

		row := cursor + padR(nameVal, cw.name-2) +
			padR(fileVal, cw.file) +
			catTag +
			padL(formatCount(s.Count), cw.count) +
			" " + barFillStyle.Render(bar) + " " +
			avgPart +
			minPart +
			padL(formatDur(s.MaxTime), cw.max) +
			padL(formatDur(s.TotalTime), cw.total) +
			errCountPart +
			errPctPart

		var rendered string
		switch {
		case i+scrollOffset == m.cursor:
			rendered = selectedRowStyle.Render(row)
		case s.ErrPct() >= hotErrPct:
			rendered = hotRowStyle.Render(row)
		case s.Errors > 0:
			rendered = errRowStyle.Render(row)
		case avgDur >= slowAvgThreshold:
			rendered = slowRowStyle.Render(row)
		default:
			rendered = rowStyle.Render(row)
		}

		sb.WriteString(rendered + "\n")
	}
	for i := len(stats); i < maxRows; i++ {
		sb.WriteString("\n")
	}

	sb.WriteString(div + "\n")
	if alerts != "" {
		n := strings.Count(alerts, "\n") + 1
		alertsHdr := alertStyle.Render(fmt.Sprintf(" ⚠  ANOMALY ALERTS (%d)", n))
		sb.WriteString(alertsHdr + "\n")
		sb.WriteString(alerts + "\n")
	}
	sb.WriteString(div + "\n")
	sb.WriteString(footer)

	return sb.String()
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
		label := fmt.Sprintf("%s %.0f%%", cat.String(), pct)
		parts = append(parts, catStyle(cat).Render(label))
	}

	line := "  " + strings.Join(parts, "    ")
	return statsStyle.Width(w).Render(line)
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
	titleText := fmt.Sprintf(" stracectl  details: %s ", s.Name)
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
	field("Errors last 60s", formatCount(s.ErrRate60s))
	field("Avg latency", formatDur(s.AvgTime()))
	field("P95 latency", formatDur(s.P95))
	field("P99 latency", formatDur(s.P99))
	field("Max latency", formatDur(s.MaxTime))
	field("Min latency", formatDur(s.MinTime))
	field("Total time", formatDur(s.TotalTime))

	if errnos := s.TopErrors(0); len(errnos) > 0 {
		section("ERROR BREAKDOWN")
		for _, ec := range errnos {
			pct := float64(ec.Count) / float64(s.Count) * 100
			dimField(ec.Errno, fmt.Sprintf("%s calls  (%.0f%%)", formatCount(ec.Count), pct))
		}
	}

	if len(s.RecentErrors) > 0 {
		section("RECENT ERROR SAMPLES")
		for _, es := range s.RecentErrors {
			ts := es.Time.Format("15:04:05")
			args := es.Args
			if args == "" {
				args = "<no data>"
			}
			if len(args) > w-42 {
				args = args[:w-45] + "…"
			}
			line := fmt.Sprintf("  %s  %-10s  %s", ts, es.Errno, args)
			sb.WriteString(detailDimStyle.Render(line) + "\n")
		}
	}

	if expl := alertExplanation(s.Name); expl != "" && s.ErrPct() >= hotErrPct {
		section("ANOMALY EXPLANATION")
		for _, line := range wordWrap(expl, w-22) {
			sb.WriteString(alertStyle.Render("  ⚠  "+line) + "\n")
		}
	}

	sb.WriteString("\n")
	sb.WriteString(div + "\n")
	sb.WriteString(footerStyle.Render(" any key to return  │  ↑↓/jk:navigate syscalls  │  q:quit "))
	return sb.String()
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

// ── Log overlay ───────────────────────────────────────────────────────────────

func (m model) renderLog() string {
	w := m.width
	if w == 0 {
		w = 80
	}
	h := m.height
	if h == 0 {
		h = 24
	}

	entries := m.agg.RecentLog()
	n := len(entries)

	div := divStyle.Render(strings.Repeat("─", w))
	title := detailTitleStyle.Width(w).Render(fmt.Sprintf(" stracectl  live log  (%d entries) ", n))
	footer := footerStyle.Render(" any key:return  ↑↓/jk:scroll  q:quit ")

	// Visible body height: total height minus title, div, footer.
	bodyH := h - 3
	if bodyH < 1 {
		bodyH = 1
	}

	// On first open (logOffset == -1) pin to the bottom.
	if m.logOffset < 0 || m.logOffset > n-bodyH {
		m.logOffset = n - bodyH
	}
	if m.logOffset < 0 {
		m.logOffset = 0
	}

	var sb strings.Builder
	sb.WriteString(title + "\n")
	sb.WriteString(div + "\n")

	end := m.logOffset + bodyH
	if end > n {
		end = n
	}
	visible := entries[m.logOffset:end]

	maxNameW := 16
	for _, e := range visible {
		ts := e.Time.Format("15:04:05.000")
		errTag := "   "
		if e.Error != "" {
			errTag = detailDimStyle.Render("ERR")
		}
		name := e.Name
		if len(name) > maxNameW {
			name = name[:maxNameW]
		}
		args := e.Args
		avail := w - 15 - maxNameW - 6 - 4 // ts + padding + name + errTag + separators
		if avail < 10 {
			avail = 10
		}
		if len(args) > avail {
			args = args[:avail-1] + "…"
		}
		line := fmt.Sprintf("%s  %s  %-*s  %s", ts, errTag, maxNameW, name, args)
		if e.Error != "" {
			sb.WriteString(errNumStyle.Render(line) + "\n")
		} else {
			sb.WriteString(detailDimStyle.Render(line) + "\n")
		}
	}
	// Pad to bodyH if fewer lines.
	for i := len(visible); i < bodyH; i++ {
		sb.WriteString("\n")
	}
	sb.WriteString(footer)
	return sb.String()
}

// renderFiles shows the top opened files overlay. Supports simple scrolling.
func (m model) renderFiles() string {
	w := m.width
	if w == 0 {
		w = 80
	}
	h := m.height
	if h == 0 {
		h = 24
	}

	files := m.agg.TopFiles(0)
	n := len(files)

	div := divStyle.Render(strings.Repeat("─", w))
	title := detailTitleStyle.Width(w).Render(fmt.Sprintf(" stracectl  top files  (%d entries) ", n))
	footer := footerStyle.Render(" any key:return  ↑↓/jk:scroll  q:quit ")

	// Visible body height: title + div + footer accounted similar to log view.
	bodyH := h - 3
	if bodyH < 1 {
		bodyH = 1
	}

	if m.filesOffset < 0 || m.filesOffset > n-bodyH {
		m.filesOffset = 0
	}
	if m.filesOffset < 0 {
		m.filesOffset = 0
	}

	end := m.filesOffset + bodyH
	if end > n {
		end = n
	}
	visible := files[m.filesOffset:end]

	var sb strings.Builder
	sb.WriteString(title + "\n")
	sb.WriteString(div + "\n")

	availPathW := w - 12
	if availPathW < 10 {
		availPathW = 10
	}

	for _, f := range visible {
		p := f.Path
		disp := truncateToWidth(p, availPathW)
		line := fmt.Sprintf("  %-*s %6s", availPathW, disp, formatCount(f.Count))
		sb.WriteString(detailDimStyle.Render(line) + "\n")
	}
	for i := len(visible); i < bodyH; i++ {
		sb.WriteString("\n")
	}
	sb.WriteString(footer)
	return sb.String()
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
	patternRow := func(key, desc string) {
		sb.WriteString(activeSortStyle.Render(fmt.Sprintf("  %-15s", key)))
		sb.WriteString(rowStyle.Render(desc) + "\n")
	}

	section("COLUMNS")
	row("SYSCALL", "name of the kernel function called by the process")
	row("FILE", "top observed file path for this syscall (truncated)")
	row("CAT", "category: I/O · FS · NET · MEM · PROC · SIG · OTHER")
	row("CALLS", "total number of times this syscall was called")
	row("FREQ", "bar showing count relative to the most-called syscall")
	row("AVG", "average time the kernel spent executing this syscall (yellow = slow ≥5ms)")
	row("MAX", "peak (worst) latency — outliers that avg hides")
	row("TOTAL", "cumulative CPU time spent inside this syscall")
	row("ERRORS", "number of calls that returned an error (red)")
	row("ERR%", "percentage of calls that returned an error (red)")

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
	patternRow("openat ERR%", "dynamic linker searches multiple paths — usually harmless")
	patternRow("recvfrom ERR%", "EAGAIN on non-blocking socket — normal for async I/O")
	patternRow("connect ERR%", "Happy Eyeballs: IPv4 and IPv6 tried in parallel, loser fails")
	patternRow("ioctl ERR%", "process has no TTY (running under sudo or piped)")
	patternRow("madvise ERR%", "memory hints rejected by kernel — informational")
	patternRow("high I/O%", "process is doing heavy file or socket data transfer")
	patternRow("high FS%", "process is scanning directories or checking many files")
	patternRow("high SIG%", "many signal handlers registered — common during lib init")

	section("KEYBOARD SHORTCUTS")
	row("↑ / k", "move selection up")
	row("↓ / j", "move selection down")
	row("enter / d", "open detail page for selected syscall")
	row("c", "sort by COUNT (most called first)")
	row("t", "sort by TOTAL time (most CPU in kernel)")
	row("a", "sort by AVG latency")
	row("x", "sort by MAX latency (find worst outlier)")
	row("e", "sort by error count")
	row("n", "sort alphabetically")
	row("g", "group by category (I/O, FS, NET, MEM, PROC, SIG, OTHER)")
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
	name, file, cat, count, bar, avg, min, max, total, errors, errpct int
}

func colWidths(w int) cols {
	// Base widths for right-side columns (fixed-width fields)
	cat, count, avg, min, max, total, errors, errpct := 6, 9, 10, 10, 10, 11, 8, 7
	barW := 12

	// Compute remaining space after the fixed right-side columns.
	fixed := cat + count + barW + 2 + avg + min + max + total + errors + errpct
	avail := w - fixed

	// Fallback for very narrow terminals: keep previous clamping behaviour.
	if avail <= 0 {
		file := 30
		name := w - file - cat - count - barW - 2 - avg - min - max - total - errors - errpct
		if name < 14 {
			diff := 14 - name
			file -= diff
			if file < 8 {
				file = 8
			}
			name = 14
		}
		return cols{name, file, cat, count, barW, avg, min, max, total, errors, errpct}
	}

	// Prefer to keep FILE reasonably large but conservative so the main
	// syscall `SYSCALL` column remains readable. Use ~60% of available
	// left-side space, with sensible min/max and a larger minimum for
	// the syscall name to avoid crushing it on medium-width terminals.
	file := avail * 60 / 100 // reserve ~60% of the available space
	if file < 30 {
		file = 30
	}
	// Ensure we leave room for a reasonable syscall name (min 18)
	if file > avail-18 {
		file = avail - 18
	}
	name := avail - file
	if name < 18 {
		diff := 18 - name
		file -= diff
		if file < 8 {
			file = 8
		}
		name = 18
	}

	return cols{name, file, cat, count, barW, avg, min, max, total, errors, errpct}
}

// truncateToWidth truncates a string to the given display width (measured by
// lipgloss.Width) and appends an ellipsis if truncated. It preserves rune
// boundaries.
func truncateToWidth(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= w {
		return s
	}
	var b strings.Builder
	for _, r := range s {
		b.WriteRune(r)
		if lipgloss.Width(b.String()) > w {
			// remove last rune
			out := []rune(b.String())
			if len(out) > 0 {
				out = out[:len(out)-1]
			}
			return string(out) + "…"
		}
	}
	return s
}

// sanitizeForTUI removes control characters that can corrupt terminal
// rendering (including NUL). It preserves printable runes and tabs.
func sanitizeForTUI(s string) string {
	if s == "" {
		return s
	}
	var b strings.Builder
	for _, r := range s {
		if r == '\t' {
			b.WriteRune(r)
			continue
		}
		if r < 32 || r == 0x7f {
			// replace control characters with U+FFFD replacement glyph
			// so they don't shift terminal layout or inject escapes.
			b.WriteRune('�')
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// detectFallbackSize tries several methods to determine a reasonable terminal
// width/height when BubbleTea doesn't deliver a WindowSizeMsg (common when
// running under sudo, piped stdout, or after a re-exec). It first attempts
// to query the stdout file descriptor, then falls back to environment
// variables `COLUMNS`/`LINES`, and finally to sensible defaults.
func detectFallbackSize() (int, int) {
	// Try using the terminal API on stdout
	if fd := int(os.Stdout.Fd()); fd >= 0 {
		if w, h, err := term.GetSize(fd); err == nil && w > 0 && h > 0 {
			recordUIEvent("detect-term-getsize", w, h)
			return w, h
		} else if err != nil {
			recordUIEvent("detect-term-getsize-failed", 0, 0)
		}
	}

	// Next, try environment variables commonly set by shells
	if s := os.Getenv("COLUMNS"); s != "" {
		if w, err := strconv.Atoi(s); err == nil && w > 0 {
			if l := os.Getenv("LINES"); l != "" {
				if h, err2 := strconv.Atoi(l); err2 == nil && h > 0 {
					return w, h
				}
			}
			return w, 24
		}
	}

	// Last resort: conservative defaults
	recordUIEvent("detect-default", 80, 24)
	return 80, 24
}

// recordFallbackEvent appends a small diagnostic entry when the UI applies
// a fallback terminal size. This helps users report occurrences where the
// WindowSizeMsg was never delivered.
func recordFallbackEvent(w, h int) {
	f, err := os.OpenFile("/tmp/stracectl_ui_fallback.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = fmt.Fprintf(f, "%s pid=%d fallback width=%d height=%d\n", time.Now().Format(time.RFC3339Nano), os.Getpid(), w, h)
}

// recordUIEvent appends a timestamped UI event to a debug log to aid
// diagnosing why the TUI sometimes doesn't receive WindowSizeMsg.
func recordUIEvent(ev string, w, h int) {
	f, err := os.OpenFile("/tmp/stracectl_ui_events.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = fmt.Fprintf(f, "%s pid=%d ev=%s width=%d height=%d\n", time.Now().Format(time.RFC3339Nano), os.Getpid(), ev, w, h)
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
			padR("FILE", cw.file) +
			padR(mark(aggregator.SortByCategory, "CAT"), cw.cat) +
			padL(mark(aggregator.SortByCount, "CALLS"), cw.count) +
			" " + padR("FREQ", cw.bar+1) +
			padL(mark(aggregator.SortByAvg, "AVG"), cw.avg) +
			padL(mark(aggregator.SortByMin, "MIN"), cw.min) +
			padL(mark(aggregator.SortByMax, "MAX"), cw.max) +
			padL(mark(aggregator.SortByTotal, "TOTAL"), cw.total) +
			padL(mark(aggregator.SortByErrors, "ERRORS"), cw.errors) +
			padL("ERR% ", cw.errpct),
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
	w := lipgloss.Width(s)
	if w >= n {
		return s
	}
	return s + strings.Repeat(" ", n-w)
}

func padL(s string, n int) string {
	w := lipgloss.Width(s)
	if w >= n {
		return s
	}
	return strings.Repeat(" ", n-w) + s
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
// done, if non-nil, should be closed when the traced process exits; the TUI
// will quit automatically so the terminal is not left in a frozen state.
func Run(agg *aggregator.Aggregator, target string, done <-chan struct{}) error {
	return runWithOpts(agg, target, done, tea.WithAltScreen())
}

// runWithOpts is the internal entry point used by Run and by tests.
// opts are forwarded to tea.NewProgram, allowing tests to inject headless
// input/output without a real TTY.
func runWithOpts(agg *aggregator.Aggregator, target string, done <-chan struct{}, opts ...tea.ProgramOption) error {
	// Prevent tracer log.Printf messages from bleeding into the alt-screen buffer.
	// All log output is discarded while the TUI owns the terminal; it is restored
	// unconditionally when the TUI exits.
	log.SetOutput(io.Discard)
	defer log.SetOutput(os.Stderr)

	m := model{
		agg:     agg,
		target:  target,
		sortBy:  aggregator.SortByCount,
		started: time.Now(),
	}
	recordUIEvent("tea-newprogram", 0, 0)
	p := tea.NewProgram(m, opts...)
	recordUIEvent("tea-newprogram-created", 0, 0)

	if done != nil {
		go func() {
			<-done
			p.Send(processDeadMsg{})
		}()
	}

	_, err := p.Run()
	if err != nil {
		recordUIEvent("tea-run-error", 0, 0)
	} else {
		recordUIEvent("tea-run-exit", 0, 0)
	}
	return err
}
