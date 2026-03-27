package widgets

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Cols mirrors the column widths structure used by the TUI.
type Cols struct {
	Name, File, Cat, Count, Bar, Avg, Min, Max, Total, Errors, ErrPct int
}

// ColWidths returns the same layout logic used in the original tui.go.
func ColWidths(w int) Cols {
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
		return Cols{name, file, cat, count, barW, avg, min, max, total, errors, errpct}
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

	return Cols{name, file, cat, count, barW, avg, min, max, total, errors, errpct}
}

// TruncateToWidth truncates a string to the given display width (measured by
// lipgloss.Width) and appends an ellipsis if truncated. It preserves rune
// boundaries.
func TruncateToWidth(s string, w int) string {
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

// PadR pads or truncates to the right using lipgloss width measurement.
func PadR(s string, n int) string {
	w := lipgloss.Width(s)
	if w >= n {
		return s
	}
	return s + strings.Repeat(" ", n-w)
}

// PadL pads to the left.
func PadL(s string, n int) string {
	w := lipgloss.Width(s)
	if w >= n {
		return s
	}
	return strings.Repeat(" ", n-w) + s
}

// SparkBar renders a simple filled/unfilled bar using block characters.
func SparkBar(count, maxCount int64, width int) string {
	if maxCount == 0 || width <= 0 {
		return strings.Repeat("░", width)
	}
	filled := int(float64(count) / float64(maxCount) * float64(width))
	if filled > width {
		filled = width
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

// WordWrap splits text into lines no longer than maxWidth characters.
func WordWrap(text string, maxWidth int) []string {
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

// SanitizeForTUI removes control characters that can corrupt terminal
// rendering (including NUL). It preserves printable runes and tabs.
func SanitizeForTUI(s string) string {
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
			b.WriteRune('�')
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// LipglossWidth is exposed for callers that used lipgloss.Width previously.
var LipglossWidth = lipgloss.Width
