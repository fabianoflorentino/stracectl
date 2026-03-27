package input

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/fabianoflorentino/stracectl/internal/aggregator"
)

func Test_HelpOverlayCloses(t *testing.T) {
	s := &State{HelpOverlay: true}
	HandleKey(s, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	if s.HelpOverlay {
		t.Fatalf("HelpOverlay should be false after any key, got true")
	}
}

func Test_DetailOverlayNavigationAndClose(t *testing.T) {
	s := &State{DetailOverlay: true, Cursor: 1}
	HandleKey(s, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if s.Cursor != 0 {
		t.Fatalf("expected cursor 0 after up, got %d", s.Cursor)
	}
	HandleKey(s, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if s.Cursor != 1 {
		t.Fatalf("expected cursor 1 after down, got %d", s.Cursor)
	}
	// any other key closes
	HandleKey(s, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	if s.DetailOverlay {
		t.Fatalf("detail overlay should be closed on other key")
	}
}

func Test_LogAndFilesOverlayScrollingAndClose(t *testing.T) {
	s := &State{LogOverlay: true, LogOffset: 2}
	HandleKey(s, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if s.LogOffset != 1 {
		t.Fatalf("expected LogOffset 1 after up, got %d", s.LogOffset)
	}
	HandleKey(s, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if s.LogOffset != 2 {
		t.Fatalf("expected LogOffset 2 after down, got %d", s.LogOffset)
	}
	// files overlay
	s = &State{FilesOverlay: true, FilesOffset: 1}
	HandleKey(s, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if s.FilesOffset != 0 {
		t.Fatalf("expected FilesOffset 0 after up, got %d", s.FilesOffset)
	}
}

func Test_SortingAndSpecialKeys(t *testing.T) {
	s := &State{SortBy: aggregator.SortByCount}
	HandleKey(s, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	if s.SortBy != aggregator.SortByName {
		t.Fatalf("expected SortByName after pressing 'n', got %v", s.SortBy)
	}
	// toggling detail/log/files (when not editing)
	s = &State{}
	HandleKey(s, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	if !s.DetailOverlay {
		t.Fatalf("expected DetailOverlay true after pressing 'd'")
	}
	s = &State{}
	HandleKey(s, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	if !s.LogOverlay {
		t.Fatalf("expected LogOverlay true after pressing 'l'")
	}
	s = &State{}
	HandleKey(s, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")})
	if !s.FilesOverlay {
		t.Fatalf("expected FilesOverlay true after pressing 'f'")
	}
}

func Test_FilterEditing(t *testing.T) {
	s := &State{}
	HandleKey(s, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	if !s.Editing {
		t.Fatalf("expected editing true after '/'")
	}
	HandleFilterKey(s, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	if s.Filter != "a" {
		t.Fatalf("expected filter 'a', got %q", s.Filter)
	}
	HandleFilterKey(s, tea.KeyMsg{Type: tea.KeyBackspace})
	if s.Filter != "" {
		t.Fatalf("expected filter empty after backspace, got %q", s.Filter)
	}
	HandleFilterKey(s, tea.KeyMsg{Type: tea.KeyEnter})
	if s.Editing {
		t.Fatalf("expected editing false after Enter")
	}
}
