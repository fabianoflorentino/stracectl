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

// RenderAlerts inspects aggregator stats and returns rendered anomaly lines.
func RenderAlerts(agg umodel.AggregatorView) string {
	stats := agg.Sorted(aggregator.SortByErrors)
	var lines []string
	for _, s := range stats {
		if s.ErrPct() >= hotErrPct {
			expl := AlertExplanation(s.Name)
			msg := fmt.Sprintf(" ⚠  %s: %.0f%% error rate (%d/%d calls)",
				s.Name, s.ErrPct(), s.Errors, s.Count)
			if expl != "" {
				msg += " — " + expl
			}
			lines = append(lines, styles.AlertStyle.Render(msg))
		} else if s.AvgTime() >= 5*time.Millisecond { // use 5ms threshold
			lines = append(lines,
				styles.SlowRowStyle.Render(fmt.Sprintf(" ⚡  %s: slow avg %s (max %s) — kernel spending time in this call",
					s.Name, formatDurShort(s.AvgTime()), formatDurShort(s.MaxTime))))
		}
	}
	return strings.Join(lines, "\n")
}

// alertExplanation gives a human-readable reason for common high-error syscalls.
func AlertExplanation(name string) string {
	switch name {
	case "ioctl":
		return "terminal control failed — process likely has no TTY (running under sudo or piped)"
	case "openat", "open":
		return "files not found — often normal (dynamic linker searches multiple paths)"
	case "access", "faccessat":
		return "optional files are missing — usually harmless (checking for config files)"
	case "connect":
		return "connection attempts failed — may be Happy Eyeballs (IPv4/IPv6 race) or no route"
	case "recvfrom", "recv", "recvmsg":
		return "EAGAIN on non-blocking socket — normal for async I/O, not a real error"
	case "sendto", "send", "sendmsg":
		return "send failed — peer may have closed the connection"
	case "madvise":
		return "memory hint rejected by kernel — informational, not a real failure"
	case "prctl":
		return "process control rejected — may lack capabilities (seccomp or container policy)"
	case "statfs", "fstatfs":
		return "filesystem stat failed — path may be on a special fs (proc, tmpfs)"
	case "unlink", "unlinkat":
		return "tried to delete a non-existent file — may be cleanup of temp files"
	case "mkdir", "mkdirat":
		return "directory already exists — common during first-run initialisation"
	default:
		return ""
	}
}

// formatDurShort is a tiny wrapper delegating to styles package formatting
// but kept here to avoid import cycles; it mirrors helpers.FormatDur.
func formatDurShort(d time.Duration) string {
	if d == 0 {
		return "—"
	}
	return d.String()
}
