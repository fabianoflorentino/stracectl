package render

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/fabianoflorentino/stracectl/internal/ui/controller"
	"github.com/fabianoflorentino/stracectl/internal/ui/helpers"
	"github.com/fabianoflorentino/stracectl/internal/ui/styles"
	"github.com/fabianoflorentino/stracectl/internal/ui/widgets"
)

var (
	titleStyle       = styles.TitleStyle
	footerStyle      = styles.FooterStyle
	filterStyle      = styles.FilterStyle
	divStyle         = styles.DivStyle
	alertStyle       = styles.AlertStyle
	rowStyle         = styles.RowStyle
	selectedRowStyle = styles.SelectedRowStyle
	errRowStyle      = styles.ErrRowStyle
	hotRowStyle      = styles.HotRowStyle
	slowRowStyle     = styles.SlowRowStyle
	barFillStyle     = styles.BarFillStyle
	errNumStyle      = styles.ErrNumStyle
	slowAvgStyle     = styles.SlowAvgStyle
)

// RenderView produces the main table view (title, category bar, rows, alerts, footer)
// by querying the provided UIController for state. It mirrors the previous ui.View
// orchestration but lives in the render package to decouple presentation.
func RenderView(ctrl controller.UIController) string {
	w := ctrl.Width()
	if w == 0 {
		return "Initialising stracectl…"
	}

	agg := ctrl.Agg()
	cw := widgets.ColWidths(w)

	// header
	total := agg.Total()
	errs := agg.Errors()
	rate := agg.Rate()
	unique := agg.UniqueCount()
	elapsed := time.Since(ctrl.Started()).Round(time.Second)

	procLabel := ctrl.Target()
	if pi := agg.GetProcInfo(); pi.Comm != "" {
		procLabel = fmt.Sprintf("%s[%d]", pi.Comm, pi.PID)
	}
	left := fmt.Sprintf(" stracectl  %s  +%s ", procLabel, elapsed)
	right := fmt.Sprintf(" syscalls: %s  rate: %.0f/s  errors: %s  unique: %d ",
		helpers.FormatCount(total), rate, helpers.FormatCount(errs), unique)
	gap := w - len(left) - len(right)
	if gap < 0 {
		gap = 0
	}
	titleLine := titleStyle.Render(left + strings.Repeat(" ", gap) + right)

	catLine := RenderCategoryBar(agg, w)

	div := divStyle.Render(strings.Repeat("─", w))
	hdr := RenderHeader(cw, ctrl.SortBy())

	shortcuts := " q:quit  c:calls▼  t:total  a:avg  m:min  x:max  e:errors  n:name  g:category  /:filter  ↑↓/jk:move  enter/d:details  l:log  f:files  ?:help  esc:clear"
	if ctrl.Filter() != "" {
		shortcuts += fmt.Sprintf("   [filter: %q]", ctrl.Filter())
	}

	var footer string
	if ctrl.Editing() {
		footer = filterStyle.Render(fmt.Sprintf(" filter: %s█", ctrl.Filter()))
	} else if ctrl.ProcessDone() {
		footer = footerStyle.Render(shortcuts)
	} else {
		footer = footerStyle.Render(shortcuts)
	}

	// alerts
	alerts := RenderAlerts(agg)

	// layout math
	footerLines := strings.Count(footer, "\n") + 1
	fixedLines := 8 + footerLines
	if alerts != "" {
		fixedLines += strings.Count(alerts, "\n") + 2
	}
	maxRows := ctrl.Height() - fixedLines
	if maxRows < 1 {
		maxRows = 1
	}

	stats := agg.Sorted(ctrl.SortBy())
	if ctrl.Filter() != "" {
		needle := strings.ToLower(ctrl.Filter())
		filtered := stats[:0]
		for _, s := range stats {
			if strings.Contains(s.Name, needle) {
				filtered = append(filtered, s)
			}
		}
		stats = filtered
	}

	// compute maxCount
	var maxCount int64
	for _, s := range stats {
		if s.Count > maxCount {
			maxCount = s.Count
		}
	}

	// clamp cursor
	cur := ctrl.Cursor()
	if cur >= len(stats) {
		cur = len(stats) - 1
	}
	if cur < 0 {
		cur = 0
	}

	// scroll offset
	scrollOffset := 0
	if len(stats) > maxRows {
		if cur >= maxRows {
			scrollOffset = cur - maxRows + 1
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
		bar := widgets.SparkBar(s.Count, maxCount, cw.Bar)

		nameVal := widgets.TruncateToWidth(widgets.SanitizeForTUI(s.Name), cw.Name-2)

		fileVal := ""
		if len(s.Files) > 0 {
			fileVal = widgets.TruncateToWidth(widgets.SanitizeForTUI(s.Files[0].Path), cw.File)
		}

		catLabel := widgets.TruncateToWidth(s.Category.String(), cw.Cat)
		catTag := catStyle(s.Category).Render(widgets.PadR(catLabel, cw.Cat))

		cursor := "  "
		if i+scrollOffset == cur {
			cursor = "► "
		}

		avgDur := s.AvgTime()
		var avgPart string
		if avgDur >= 5*time.Millisecond {
			durStr := helpers.FormatDur(avgDur)
			pad := max(0, cw.Avg-lipgloss.Width(durStr))
			avgPart = strings.Repeat(" ", pad) + slowAvgStyle.Render(durStr)
		} else {
			avgPart = widgets.PadL(helpers.FormatDur(avgDur), cw.Avg)
		}

		minPart := widgets.PadL(helpers.FormatDur(s.MinTime), cw.Min)

		var errCountPart, errPctPart string
		if s.Errors > 0 {
			cs := helpers.FormatCount(s.Errors)
			ps := fmt.Sprintf("%.0f%%", s.ErrPct())
			errCountPart = strings.Repeat(" ", max(0, cw.Errors-len(cs))) + errNumStyle.Render(cs)
			errPctPart = strings.Repeat(" ", max(0, cw.ErrPct-len(ps))) + errNumStyle.Render(ps)
		} else {
			errCountPart = widgets.PadL("—", cw.Errors)
			errPctPart = widgets.PadL("—", cw.ErrPct)
		}

		row := cursor + widgets.PadR(nameVal, cw.Name-2) +
			widgets.PadR(fileVal, cw.File) +
			catTag +
			widgets.PadL(helpers.FormatCount(s.Count), cw.Count) +
			" " + barFillStyle.Render(bar) + " " +
			avgPart +
			minPart +
			widgets.PadL(helpers.FormatDur(s.MaxTime), cw.Max) +
			widgets.PadL(helpers.FormatDur(s.TotalTime), cw.Total) +
			errCountPart +
			errPctPart

		var rendered string
		switch {
		case i+scrollOffset == cur:
			rendered = selectedRowStyle.Render(row)
		case s.ErrPct() >= 50.0:
			rendered = hotRowStyle.Render(row)
		case s.Errors > 0:
			rendered = errRowStyle.Render(row)
		case avgDur >= 5*time.Millisecond:
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
