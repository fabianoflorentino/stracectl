package input

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func Test_HandleKey_OverlaysAndShortcuts(t *testing.T) {
	s := &State{}
	// Help overlay closes on any key
	s.HelpOverlay = true
	HandleKey(s, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if s.HelpOverlay {
		t.Fatal("HelpOverlay should be false after key")
	}

	// Detail overlay navigation and quit
	s.DetailOverlay = true
	s.Cursor = 1
	HandleKey(s, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}) // up
	if s.Cursor != 0 {
		t.Fatalf("expected cursor 0, got %d", s.Cursor)
	}
	// pressing q in detail overlay returns Quit
	s.DetailOverlay = true
	if cmd := HandleKey(s, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}); cmd == nil {
		t.Fatal("expected non-nil command when pressing q in detail overlay")
	}

	// Log overlay scroll
	s.DetailOverlay = false
	s.LogOverlay = true
	s.LogOffset = 1
	HandleKey(s, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if s.LogOffset != 0 {
		t.Fatalf("expected logoffset 0, got %d", s.LogOffset)
	}

	// Files overlay scroll
	s.DetailOverlay = false
	s.LogOverlay = false
	s.FilesOverlay = true
	s.FilesOffset = 1
	HandleKey(s, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if s.FilesOffset != 2 {
		t.Fatalf("expected filesoffset 2, got %d", s.FilesOffset)
	}

	// Global shortcuts: ?, /, esc, sorting keys
	s = &State{}
	HandleKey(s, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if !s.HelpOverlay {
		t.Fatal("expected HelpOverlay true")
	}
	HandleKey(s, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	HandleKey(s, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if s.SortBy != 0 {
		t.Fatalf("expected SortBy to be default (0), got %v", s.SortBy)
	}
}

func Test_HandleFilterKey_EditAndBackspace(t *testing.T) {
	s := &State{Editing: true, Filter: ""}
	HandleFilterKey(s, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a', 'b'}})
	if s.Filter != "ab" {
		t.Fatalf("expected filter 'ab', got %q", s.Filter)
	}
	HandleFilterKey(s, tea.KeyMsg{Type: tea.KeyBackspace})
	if s.Filter != "a" {
		t.Fatalf("expected filter 'a', got %q", s.Filter)
	}
	HandleFilterKey(s, tea.KeyMsg{Type: tea.KeyEnter})
	if s.Editing {
		t.Fatalf("expected editing false after Enter")
	}
}
