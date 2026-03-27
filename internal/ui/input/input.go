package input

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/fabianoflorentino/stracectl/internal/aggregator"
)

// State represents the minimal mutable UI state needed for input handling.
type State struct {
	HelpOverlay   bool
	DetailOverlay bool
	LogOverlay    bool
	FilesOverlay  bool

	Editing     bool
	Filter      string
	Cursor      int
	LogOffset   int
	FilesOffset int
	SortBy      aggregator.SortField
	ProcessDone bool
}

// HandleKey updates the provided state according to the incoming key message
// and returns an optional tea.Cmd (e.g., Quit).
func HandleKey(s *State, msg tea.KeyMsg) tea.Cmd {
	// any key closes the help overlay
	if s.HelpOverlay {
		s.HelpOverlay = false
		return nil
	}
	// in the detail overlay: navigate or close
	if s.DetailOverlay {
		switch msg.String() {
		case "up", "k":
			if s.Cursor > 0 {
				s.Cursor--
			}
		case "down", "j":
			s.Cursor++ // clamped elsewhere
		case "q", "Q", "ctrl+c":
			return tea.Quit
		default:
			s.DetailOverlay = false
		}
		return nil
	}
	// in the log overlay: scroll or close
	if s.LogOverlay {
		switch msg.String() {
		case "up", "k":
			if s.LogOffset > 0 {
				s.LogOffset--
			}
		case "down", "j":
			s.LogOffset++
		case "q", "Q", "ctrl+c":
			return tea.Quit
		default:
			s.LogOverlay = false
		}
		return nil
	}
	// in the files overlay: scroll or close
	if s.FilesOverlay {
		switch msg.String() {
		case "up", "k":
			if s.FilesOffset > 0 {
				s.FilesOffset--
			}
		case "down", "j":
			s.FilesOffset++
		case "q", "Q", "ctrl+c":
			return tea.Quit
		default:
			s.FilesOverlay = false
		}
		return nil
	}
	switch msg.String() {
	case "q", "Q", "ctrl+c":
		return tea.Quit
	case "?":
		s.HelpOverlay = true
	case "/":
		s.Editing = true
		s.Filter = ""
	case "esc":
		s.Filter = ""
		s.Cursor = 0
	case "c":
		s.SortBy = aggregator.SortByCount
	case "t":
		s.SortBy = aggregator.SortByTotal
	case "a":
		s.SortBy = aggregator.SortByAvg
	case "m":
		s.SortBy = aggregator.SortByMin
	case "x":
		s.SortBy = aggregator.SortByMax
	case "e":
		s.SortBy = aggregator.SortByErrors
	case "n":
		s.SortBy = aggregator.SortByName
	case "g":
		s.SortBy = aggregator.SortByCategory
	case "up", "k":
		if s.Cursor > 0 {
			s.Cursor--
		}
	case "down", "j":
		s.Cursor++ // clamped by renderer
	case "d", "D", "enter", " ":
		s.DetailOverlay = true
	case "l", "L":
		s.LogOverlay = true
		s.LogOffset = -1 // -1 signals "scroll to bottom" on first render
	case "f", "F":
		s.FilesOverlay = true
		s.FilesOffset = 0
	}
	return nil
}

// HandleFilterKey processes keys while in filter (editing) mode.
func HandleFilterKey(s *State, msg tea.KeyMsg) tea.Cmd {
	switch msg.Type {
	case tea.KeyEscape, tea.KeyEnter:
		s.Editing = false
	case tea.KeyBackspace:
		if len(s.Filter) > 0 {
			s.Filter = s.Filter[:len(s.Filter)-1]
		}
	case tea.KeyRunes:
		s.Filter += string(msg.Runes)
	}
	return nil
}
