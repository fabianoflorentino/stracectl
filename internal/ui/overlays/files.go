package overlays

import (
	"fmt"
	"strings"

	umodel "github.com/fabianoflorentino/stracectl/internal/ui/model"
	"github.com/fabianoflorentino/stracectl/internal/ui/styles"
)

// RenderFiles shows the top opened files overlay. Accepts filesOffset pointer
// so the caller can preserve scroll position.
func RenderFiles(w, h int, agg umodel.AggregatorView, filesOffset *int) string {
	if w == 0 {
		w = 80
	}
	if h == 0 {
		h = 24
	}

	files := agg.TopFiles(0)
	n := len(files)

	div := styles.DivStyle.Render(stringsRepeat("─", w))
	title := styles.DetailTitleStyle.Width(w).Render(fmt.Sprintf(" stracectl  top files  (%d entries) ", n))
	footer := styles.FooterStyle.Render(" any key:return  ↑↓/jk:scroll  q:quit ")

	bodyH := max(h-3, 1)

	if *filesOffset < 0 || *filesOffset > n-bodyH {
		*filesOffset = 0
	}

	end := min(*filesOffset+bodyH, n)
	visible := files[*filesOffset:end]

	var sb strings.Builder

	sb.WriteString(title + "\n")
	sb.WriteString(div + "\n")

	availPathW := max(w-12, 10)

	for _, f := range visible {
		p := f.Path
		disp := p

		if len(disp) > availPathW {
			disp = disp[:availPathW]
		}

		line := fmt.Sprintf("  %-*s %6s", availPathW, disp, helpersFormatCount(f.Count))
		sb.WriteString(styles.DetailDimStyle.Render(line) + "\n")
	}

	for i := len(visible); i < bodyH; i++ {
		sb.WriteString("\n")
	}

	sb.WriteString(footer)

	return sb.String()
}

// stringsRepeat is defined in log.go; reuse common helper there. Omit duplicate.

// small local helper to avoid importing the helpers package here
func helpersFormatCount(n int64) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}
