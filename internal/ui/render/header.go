package render

import (
	"github.com/fabianoflorentino/stracectl/internal/aggregator"
	"github.com/fabianoflorentino/stracectl/internal/ui/styles"
	"github.com/fabianoflorentino/stracectl/internal/ui/widgets"
)

// RenderHeader renders the table header using provided column widths, sort field,
// and per-PID mode. In per-PID mode the FILE column is relabelled to PID.
func RenderHeader(cw widgets.Cols, sortBy aggregator.SortField, perPID bool) string {
	mark := func(f aggregator.SortField, label string) string {
		if sortBy == f {
			return styles.ActiveSortStyle.Render(label + "▼")
		}
		return label + " "
	}

	fileLabel := "FILE"
	if perPID {
		fileLabel = "PID"
	}

	return styles.HeaderStyle.Render(
		widgets.PadR("SYSCALL", cw.Name) +
			widgets.PadR(fileLabel, cw.File) +
			widgets.PadR(mark(aggregator.SortByCategory, "CAT"), cw.Cat) +
			widgets.PadL(mark(aggregator.SortByCount, "CALLS"), cw.Count) +
			" " + widgets.PadR("FREQ", cw.Bar+1) +
			widgets.PadL(mark(aggregator.SortByAvg, "AVG"), cw.Avg) +
			widgets.PadL(mark(aggregator.SortByMin, "MIN"), cw.Min) +
			widgets.PadL(mark(aggregator.SortByMax, "MAX"), cw.Max) +
			widgets.PadL(mark(aggregator.SortByTotal, "TOTAL"), cw.Total) +
			widgets.PadL(mark(aggregator.SortByErrors, "ERRORS"), cw.Errors) +
			widgets.PadL("ERR% ", cw.ErrPct),
	)
}
