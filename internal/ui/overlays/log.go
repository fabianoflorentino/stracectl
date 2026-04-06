package overlays

import (
	"fmt"
	"strings"

	umodel "github.com/fabianoflorentino/stracectl/internal/ui/model"
	"github.com/fabianoflorentino/stracectl/internal/ui/styles"
)

// RenderLog renders the live log overlay. It accepts a pointer to offset so
// callers (e.g., the main model) can keep the scroll position.
func RenderLog(w, h int, agg umodel.AggregatorView, offset *int) string {
	if w == 0 {
		w = 80
	}
	if h == 0 {
		h = 24
	}

	entries := agg.RecentLog()
	n := len(entries)

	div := styles.DivStyle.Render(stringsRepeat("─", w))
	title := styles.DetailTitleStyle.Width(w).Render(fmt.Sprintf(" stracectl  live log  (%d entries) ", n))
	footer := styles.FooterStyle.Render(" any key:return  ↑↓/jk:scroll  q:quit ")

	bodyH := max(h-3, 1)

	if *offset < 0 || *offset > n-bodyH {
		*offset = n - bodyH
	}
	if *offset < 0 {
		*offset = 0
	}

	var sb strings.Builder
	sb.WriteString(title + "\n")
	sb.WriteString(div + "\n")

	end := min(*offset+bodyH, n)
	visible := entries[*offset:end]

	maxNameW := 16
	for _, e := range visible {
		ts := e.Time.Format("15:04:05.000")
		errTag := "   "

		if e.Error != "" {
			errTag = styles.DetailDimStyle.Render("ERR")
		}

		name := e.Name
		if len(name) > maxNameW {
			name = name[:maxNameW]
		}

		args := e.Args
		avail := max(w-15-maxNameW-6-4, 10)
		if len(args) > avail {
			args = args[:avail-1] + "…"
		}

		line := fmt.Sprintf("%s  %s  %-*s  %s", ts, errTag, maxNameW, name, args)
		if e.Error != "" {
			sb.WriteString(styles.ErrNumStyle.Render(line) + "\n")
		} else {
			sb.WriteString(styles.DetailDimStyle.Render(line) + "\n")
		}
	}

	for i := len(visible); i < bodyH; i++ {
		sb.WriteString("\n")
	}

	sb.WriteString(footer)

	return sb.String()
}

// stringsRepeat is a helper that safely repeats a string n times, returning an
// empty string if n is non-positive.
func stringsRepeat(s string, n int) string {
	if n <= 0 {
		return ""
	}

	return strings.Repeat(s, n)
}
