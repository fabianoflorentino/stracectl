package render

import (
	"fmt"
	"strings"

	"github.com/fabianoflorentino/stracectl/internal/aggregator"
	"github.com/fabianoflorentino/stracectl/internal/ui/helpers"
	umodel "github.com/fabianoflorentino/stracectl/internal/ui/model"
	"github.com/fabianoflorentino/stracectl/internal/ui/styles"
	"github.com/fabianoflorentino/stracectl/internal/ui/widgets"
)

var (
	headerStyle      = styles.HeaderStyle
	detailTitleStyle = styles.DetailTitleStyle
	detailLabelStyle = styles.DetailLabelStyle
	detailValueStyle = styles.DetailValueStyle
	detailDimStyle   = styles.DetailDimStyle
	detailCodeStyle  = styles.DetailCodeStyle
)

// RenderDetail builds the detail overlay for the selected syscall. It mirrors
// the previous ui.renderDetail behaviour but lives in the render package so
// the TUI can delegate rendering responsibilities.
func RenderDetail(agg umodel.AggregatorView, sortBy aggregator.SortField, filter string, cursor, w, h int) string {
	if w == 0 {
		w = 80
	}

	stats := agg.Sorted(sortBy)
	if filter != "" {
		needle := strings.ToLower(filter)
		filtered := stats[:0]
		for _, s := range stats {
			if strings.Contains(s.Name, needle) {
				filtered = append(filtered, s)
			}
		}
		stats = filtered
	}

	if len(stats) == 0 {
		return detailDimStyle.Render("  no syscall selected")
	}
	idx := cursor
	if idx >= len(stats) {
		idx = len(stats) - 1
	}
	s := stats[idx]

	div := divStyle.Render(strings.Repeat("─", w))

	var sb strings.Builder
	titleText := fmt.Sprintf(" stracectl  details: %s ", s.Name)
	sb.WriteString(detailTitleStyle.Width(w).Render(titleText) + "\n")
	sb.WriteString(div + "\n")

	info := SyscallInfo(s.Name)

	field := func(label, value string) {
		l := detailLabelStyle.Render(fmt.Sprintf("  %-18s", label))
		v := detailValueStyle.Render(value)
		sb.WriteString(l + v + "\n")
	}
	dimField := func(label, value string) {
		l := detailLabelStyle.Render(fmt.Sprintf("  %-18s", label))
		v := detailDimStyle.Render(value)
		sb.WriteString(l + v + "\n")
	}
	codeField := func(label, value string) {
		l := detailLabelStyle.Render(fmt.Sprintf("  %-18s", label))
		v := detailCodeStyle.Render(value)
		sb.WriteString(l + v + "\n")
	}
	section := func(title string) {
		sb.WriteString("\n")
		sb.WriteString(headerStyle.Render(" "+title) + "\n")
		sb.WriteString(div + "\n")
	}

	section("SYSCALL REFERENCE")
	field("Name", s.Name)
	field("Category", catStyle(s.Category).Render(s.Category.String()))
	field("Description", info.Description)

	if info.Signature != "" {
		codeField("Signature", info.Signature)
	}

	if len(info.Args) > 0 {
		section("ARGUMENTS")
		for _, a := range info.Args {
			dimField(a[0], a[1])
		}
	}

	if info.ReturnValue != "" {
		section("RETURN VALUE")
		field("On success", info.ReturnValue)
		if info.ErrorHint != "" {
			field("On error", "-1, errno set")
			field("Common errors", info.ErrorHint)
		}
	}

	if info.Notes != "" {
		section("NOTES")
		wrapWidth := w - 22
		if wrapWidth < 40 {
			wrapWidth = 40
		}
		for _, line := range widgets.WordWrap(info.Notes, wrapWidth) {
			sb.WriteString(detailValueStyle.Render("  "+strings.Repeat(" ", 18)+line) + "\n")
		}
	}

	section("LIVE STATISTICS")
	field("Calls", helpers.FormatCount(s.Count))
	field("Errors", fmt.Sprintf("%s  (%.0f%%)", helpers.FormatCount(s.Errors), s.ErrPct()))
	field("Errors last 60s", helpers.FormatCount(s.ErrRate60s))
	field("Avg latency", helpers.FormatDur(s.AvgTime()))
	field("P95 latency", helpers.FormatDur(s.P95))
	field("P99 latency", helpers.FormatDur(s.P99))
	field("Max latency", helpers.FormatDur(s.MaxTime))
	field("Min latency", helpers.FormatDur(s.MinTime))
	field("Total time", helpers.FormatDur(s.TotalTime))

	if errnos := s.TopErrors(0); len(errnos) > 0 {
		section("ERROR BREAKDOWN")
		for _, ec := range errnos {
			pct := float64(ec.Count) / float64(s.Count) * 100
			dimField(ec.Errno, fmt.Sprintf("%s calls  (%.0f%%)", helpers.FormatCount(ec.Count), pct))
		}
	}

	if len(s.RecentErrors) > 0 {
		section("RECENT ERROR SAMPLES")
		for _, es := range s.RecentErrors {
			ts := es.Time.Format("15:04:05")
			args := es.Args
			if args == "" {
				args = "<no data>"
			}
			if len(args) > w-42 {
				args = args[:w-45] + "…"
			}
			line := fmt.Sprintf("  %s  %-10s  %s", ts, es.Errno, args)
			sb.WriteString(detailDimStyle.Render(line) + "\n")
		}
	}

	if expl := AlertExplanation(s.Name); expl != "" && s.ErrPct() >= 50.0 {
		section("ANOMALY EXPLANATION")
		for _, line := range widgets.WordWrap(expl, w-22) {
			sb.WriteString(alertStyle.Render("  ⚠  "+line) + "\n")
		}
	}

	sb.WriteString("\n")
	sb.WriteString(div + "\n")
	sb.WriteString(footerStyle.Render(" any key to return  │  ↑↓/jk:navigate syscalls  │  q:quit "))
	return sb.String()
}
