// Package ui provides the BubbleTea TUI for stracectl.
package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"io"
	"log"
	"os"

	"github.com/fabianoflorentino/stracectl/internal/aggregator"
	umodel "github.com/fabianoflorentino/stracectl/internal/ui/model"
	"github.com/fabianoflorentino/stracectl/internal/ui/overlays"
	"github.com/fabianoflorentino/stracectl/internal/ui/render"
	"github.com/fabianoflorentino/stracectl/internal/ui/terminal"
)

const (
	refreshInterval  = 200 * time.Millisecond
	slowAvgThreshold = 5 * time.Millisecond // avg latency considered slow
	hotErrPct        = 50.0                 // % errors considered critical
)

// ── Styles ────────────────────────────────────────────────────────────────────
var (
// Styles moved to internal/ui/styles package. Use styles.* instead.
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
	readyCh       chan<- struct{}
}

func (m *model) Init() tea.Cmd { return tick() }

// processDeadMsg is sent to the program when the traced process exits.
// The TUI marks itself as done and shows a banner, but does NOT quit
// automatically — the user reviews the final data and presses q.
type processDeadMsg struct{}

// Export small types so other packages can interact with the model when needed.
// Provide constructors/wrappers in the ui package for the app layer.
type ProcessDeadMsg = processDeadMsg

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case processDeadMsg:
		m.processDone = true
		return m, nil
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// record reception of a WindowSizeMsg for later diagnosis
		terminal.RecordUIEvent("window-size", m.width, m.height)
		if m.readyCh != nil {
			// signal readiness exactly once
			close(m.readyCh)
			m.readyCh = nil
		}
	case tickMsg:
		// If we never received a WindowSizeMsg, the TUI would stay stuck
		// showing "Initialising stracectl…". After a few ticks, attempt a
		// fallback terminal-size detection so the UI can render in degraded
		// environments (sudo, re-exec, or piped stdout) instead of freezing.
		if m.width == 0 {
			m.sizeMisses++
			if m.sizeMisses >= 5 {
				w, h := terminal.DetectFallbackSize()
				m.width = w
				m.height = h
				// record diagnostic hint for remote debugging
				terminal.RecordFallbackEvent(w, h)
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

// ModelFromAggregator builds a model prepopulated from the provided aggregator.
// Exposed so the app runner can instantiate the TUI model without duplicating
// construction logic.
func ModelFromAggregator(agg *aggregator.Aggregator, target string, ready chan<- struct{}) model {
	return model{
		agg:     agg,
		target:  target,
		sortBy:  aggregator.SortByCount,
		started: time.Now(),
		readyCh: ready,
	}
}

func (m *model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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

func (m *model) handleFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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

func (m *model) View() string {
	// delegate rendering to the specialized render/overlays packages
	if m.width == 0 {
		return "Initialising stracectl…"
	}
	if m.helpOverlay {
		return overlays.RenderHelp(m.width)
	}
	if m.detailOverlay {
		return render.RenderDetail(m.agg, m.sortBy, m.filter, m.cursor, m.width, m.height)
	}
	if m.filesOverlay {
		return overlays.RenderFiles(m.width, m.height, m.agg, &m.filesOffset)
	}
	if m.logOverlay {
		return overlays.RenderLog(m.width, m.height, m.agg, &m.logOffset)
	}

	// otherwise render the main table via RenderView using the controller adapter
	return render.RenderView(m)
}

// Controller adapter methods — implement controller.UIController for render.RenderView
func (m *model) Width() int                   { return m.width }
func (m *model) Height() int                  { return m.height }
func (m *model) Agg() umodel.AggregatorView   { return m.agg }
func (m *model) SortBy() aggregator.SortField { return m.sortBy }
func (m *model) Filter() string               { return m.filter }
func (m *model) Editing() bool                { return m.editing }
func (m *model) ProcessDone() bool            { return m.processDone }
func (m *model) Cursor() int                  { return m.cursor }
func (m *model) LogOffsetPtr() *int           { return &m.logOffset }
func (m *model) FilesOffsetPtr() *int         { return &m.filesOffset }
func (m *model) Started() time.Time           { return m.started }
func (m *model) Target() string               { return m.target }

// Run starts the full-screen TUI backed by agg.
// done, if non-nil, should be closed when the traced process exits; the TUI
// will quit automatically so the terminal is not left in a frozen state.
func Run(agg *aggregator.Aggregator, target string, done <-chan struct{}, ready chan<- struct{}) error {
	return runWithOpts(agg, target, done, ready, tea.WithAltScreen())
}

// runWithOpts is the internal entry point used by Run and by tests.
// opts are forwarded to tea.NewProgram, allowing tests to inject headless
// input/output without a real TTY.
func runWithOpts(agg *aggregator.Aggregator, target string, done <-chan struct{}, ready chan<- struct{}, opts ...tea.ProgramOption) error {
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
		readyCh: ready,
	}
	terminal.RecordUIEvent("tea-newprogram", 0, 0)
	p := tea.NewProgram(&m, opts...)
	terminal.RecordUIEvent("tea-newprogram-created", 0, 0)

	if done != nil {
		go func() {
			<-done
			p.Send(processDeadMsg{})
		}()
	}

	_, err := p.Run()
	if err != nil {
		terminal.RecordUIEvent("tea-run-error", 0, 0)
	} else {
		terminal.RecordUIEvent("tea-run-exit", 0, 0)
	}
	return err
}

// NOTE: rendering and overlay helpers moved to internal/ui/render and
// internal/ui/overlays packages. The TUI now delegates rendering to those
// packages (see View() above) and keeps only the BubbleTea plumbing here.

// render.Header, widgets and helpers provide header, sparkbars and formatters.
// Local implementations were removed in favor of the new packages.

// ── Entry point ───────────────────────────────────────────────────────────────

// Run starts the full-screen TUI backed by agg.
// done, if non-nil, should be closed when the traced process exits; the TUI
// will quit automatically so the terminal is not left in a frozen state.
// Run and runWithOpts moved to internal/ui/app to decouple entry plumbing.
