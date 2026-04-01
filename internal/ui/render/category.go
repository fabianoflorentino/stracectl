package render

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/fabianoflorentino/stracectl/internal/aggregator"
	umodel "github.com/fabianoflorentino/stracectl/internal/ui/model"
	"github.com/fabianoflorentino/stracectl/internal/ui/styles"
)

// RenderCategoryBar generates a horizontal bar showing the percentage breakdown of syscall categories.
func RenderCategoryBar(agg umodel.AggregatorView, w int) string {
	bd := agg.CategoryBreakdown()
	total := agg.Total()

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

	return styles.StatsStyle.Width(w).Render(line)
}

// catStyle returns the appropriate style for a given syscall category. This is used to color-code
// the category labels in the category breakdown bar.
func catStyle(c aggregator.Category) lipgloss.Style {
	switch c {
	case aggregator.CatIO:
		return styles.CatIOStyle
	case aggregator.CatFS:
		return styles.CatFSStyle
	case aggregator.CatNet:
		return styles.CatNetStyle
	case aggregator.CatMem:
		return styles.CatMemStyle
	case aggregator.CatProcess:
		return styles.CatProcStyle
	case aggregator.CatSignal:
		return styles.CatSigStyle
	default:
		return styles.CatOthStyle
	}
}
