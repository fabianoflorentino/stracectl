package render

import (
	"fmt"
	"strings"
	"time"

	"github.com/fabianoflorentino/stracectl/internal/aggregator"
	umodel "github.com/fabianoflorentino/stracectl/internal/ui/model"
	"github.com/fabianoflorentino/stracectl/internal/ui/styles"
)

const hotErrPct = 50.0

// RenderAlerts generates alert messages for syscalls with high error rates or slow average times.
// It uses a simple threshold of 50% error rate to flag syscalls as "hot" and includes an explanation
// for common syscalls.
func RenderAlerts(agg umodel.AggregatorView) string {
	stats := agg.Sorted(aggregator.SortByErrors)

	var lines []string

	for _, s := range stats {
		if s.ErrPct() >= hotErrPct {
			expl := AlertExplanation(s.Name)
			msg := fmt.Sprintf(" ⚠  %s: %.0f%% error rate (%d/%d calls)", s.Name, s.ErrPct(), s.Errors, s.Count)
			if expl != "" {
				msg += " — " + expl
			}

			lines = append(lines, styles.AlertStyle.Render(msg))
		} else if s.AvgTime() >= 5*time.Millisecond { // use 5ms threshold
			lines = append(lines, styles.SlowRowStyle.Render(fmt.Sprintf(" ⚡  %s: slow avg %s (max %s) — kernel spending time in this call", s.Name, formatDurShort(s.AvgTime()), formatDurShort(s.MaxTime))))
		}
	}

	return strings.Join(lines, "\n")
}

// AlertExplanation gives a human-readable reason for common high-error syscalls.
// Implementation uses a pre-initialized map for exact matches and a small
// ordered list of matchers for common variants (prefix/suffix). This avoids
// allocations per-call and makes the matching logic easier to extend.
type explanationMatcher struct {
	match func(string) bool
	expl  string
}

var (
	// explanation messages extracted to constants for easier maintenance/translation
	alertExplanations = map[string]string{
		"ioctl":    explIoctl,
		"open":     explOpen,
		"access":   explAccess,
		"connect":  explConnect,
		"recvfrom": explRecvFrom,
		"sendto":   explSendTo,
		"madvise":  explMadvise,
		"prctl":    explPrctl,
		"statfs":   explStatfs,
		"unlink":   explUnlink,
		"mkdir":    explMkdir,
	}
	alertMatchers = []explanationMatcher{
		{func(n string) bool { return n == "ioctl" }, alertExplanations["ioctl"]},
		{func(n string) bool { return n == "faccessat" }, alertExplanations["access"]},
		{func(n string) bool { return strings.HasPrefix(n, "access") }, alertExplanations["access"]},
		{func(n string) bool { return n == "connect" }, alertExplanations["connect"]},
		{func(n string) bool { return strings.HasSuffix(n, "statfs") }, alertExplanations["statfs"]},
		{func(n string) bool { return strings.HasPrefix(n, "recv") }, alertExplanations["recvfrom"]},
		{func(n string) bool { return strings.HasPrefix(n, "send") }, alertExplanations["sendto"]},
		{func(n string) bool { return n == "madvise" }, alertExplanations["madvise"]},
		{func(n string) bool { return n == "prctl" }, alertExplanations["prctl"]},
		{func(n string) bool { return strings.HasPrefix(n, "open") }, alertExplanations["open"]}, // prefix "open" covers open, openat and similar variants
		{func(n string) bool { return strings.HasPrefix(n, "unlink") }, alertExplanations["unlink"]},
		{func(n string) bool { return strings.HasPrefix(n, "mkdir") }, alertExplanations["mkdir"]},
	}
)

func AlertExplanation(name string) string {
	// exact match first
	if expl, ok := alertExplanations[name]; ok {
		return expl
	}

	// run ordered matchers (more specific rules should come earlier)
	for _, m := range alertMatchers {
		if m.match(name) {
			return m.expl
		}
	}

	return ""
}

// formatDurShort is a tiny wrapper delegating to styles package formatting
// but kept here to avoid import cycles; it mirrors helpers.FormatDur.
func formatDurShort(d time.Duration) string {
	if d == 0 {
		return "—"
	}

	return d.String()
}
