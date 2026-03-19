// Package report generates a self-contained HTML file from aggregator data.
package report

import (
	_ "embed"
	"fmt"
	"html/template"
	"math"
	"os"
	"sort"
	"time"

	"github.com/fabianoflorentino/stracectl/internal/aggregator"
)

//go:embed static/report.html
var reportHTML string

// errnoItem is a single errno + formatted stats for the report template.
type errnoItem struct {
	Errno  string
	Count  int64
	PctStr string
}

// rowData holds pre-formatted values for one table row.
type rowData struct {
	Name           string
	Category       string
	Count          int64
	FreqPct        string
	Avg            string
	Min            string
	Max            string
	Total          string
	ErrPct         string
	Errors         int64
	ErrnoBreakdown []errnoItem
}

type catRow struct {
	Name     string
	Count    int64
	Errors   int64
	BarStyle template.CSS // pre-formatted CSS value, e.g. "width:75.3%" — controlled by us, not user input
}

// data is the view-model fed to the HTML template.
type data struct {
	Label       string
	GeneratedAt string
	Duration    string
	Total       int64
	Errors      int64
	Unique      int
	ErrPct      string
	Stats       []rowData
	Categories  []catRow
	TopFiles    []aggregator.FileStat
}

// Write renders a self-contained HTML report to path.
func Write(path string, agg *aggregator.Aggregator, label string, topFilesLimit int) error {
	now := time.Now()
	duration := now.Sub(agg.StartTime())

	raw := agg.Sorted(aggregator.SortByCount)
	total := agg.Total()
	errors := agg.Errors()
	unique := agg.UniqueCount()

	rows := make([]rowData, len(raw))
	for i, s := range raw {
		var errnoBreakdown []errnoItem
		for _, ec := range s.TopErrors(0) {
			errnoBreakdown = append(errnoBreakdown, errnoItem{
				Errno:  ec.Errno,
				Count:  ec.Count,
				PctStr: fmtPct(float64(ec.Count) / float64(s.Count) * 100),
			})
		}
		rows[i] = rowData{
			Name:           s.Name,
			Category:       s.Category.String(),
			Count:          s.Count,
			FreqPct:        fmtPct(pct(s.Count, total)),
			Avg:            fmtDur(s.AvgTime()),
			Min:            fmtDur(s.MinTime),
			Max:            fmtDur(s.MaxTime),
			Total:          fmtDur(s.TotalTime),
			ErrPct:         fmtPct(s.ErrPct()),
			Errors:         s.Errors,
			ErrnoBreakdown: errnoBreakdown,
		}
	}

	bd := agg.CategoryBreakdown()
	type kv struct {
		cat aggregator.Category
		cs  aggregator.CategoryStats
	}
	pairs := make([]kv, 0, len(bd))
	for cat, cs := range bd {
		pairs = append(pairs, kv{cat, cs})
	}
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].cs.Count > pairs[j].cs.Count
	})
	var maxCat int64
	for _, p := range pairs {
		if p.cs.Count > maxCat {
			maxCat = p.cs.Count
		}
	}
	cats := make([]catRow, len(pairs))
	for i, p := range pairs {
		barPct := 0.0
		if maxCat > 0 {
			barPct = float64(p.cs.Count) / float64(maxCat) * 100
		}
		cats[i] = catRow{
			Name:     p.cat.String(),
			Count:    p.cs.Count,
			Errors:   p.cs.Errs,
			BarStyle: template.CSS(fmt.Sprintf("width:%.1f%%", math.Round(barPct*10)/10)), // #nosec G203 — value is generated internally, not from user input
		}
	}

	d := data{
		Label:       label,
		GeneratedAt: now.Format(time.RFC1123),
		Duration:    duration.Round(time.Millisecond).String(),
		Total:       total,
		Errors:      errors,
		Unique:      unique,
		ErrPct:      fmtPct(pct(errors, total)),
		Stats:       rows,
		Categories:  cats,
	}

	// Top files (most opened paths) — include in the report for quick I/O hotspots.
	// Respect requested limit; default to 50 when unspecified.
	if topFilesLimit <= 0 {
		topFilesLimit = 50
	}
	topFiles := agg.TopFiles(topFilesLimit)
	// attach to view-model
	d.TopFiles = topFiles

	tmpl, err := template.New("report").Parse(reportHTML)
	if err != nil {
		return err
	}

	f, err := os.Create(path) // #nosec G304 — path comes from a CLI argument supplied by the operator
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	return tmpl.Execute(f, d)
}

func pct(part, total int64) float64 {
	if total == 0 {
		return 0
	}
	return float64(part) / float64(total) * 100
}

func fmtDur(d time.Duration) string {
	if d == 0 {
		return "-"
	}
	switch {
	case d < time.Microsecond:
		return fmt.Sprintf("%dns", d.Nanoseconds())
	case d < time.Millisecond:
		return fmt.Sprintf("%.1fus", float64(d.Nanoseconds())/1000)
	case d < time.Second:
		return fmt.Sprintf("%.2fms", float64(d.Nanoseconds())/1e6)
	default:
		return d.Round(time.Millisecond).String()
	}
}

func fmtPct(v float64) string {
	return fmt.Sprintf("%.1f%%", v)
}
