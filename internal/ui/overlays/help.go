package overlays

import (
	"fmt"
	"strings"

	"github.com/fabianoflorentino/stracectl/internal/ui/styles"
)

// helpRow is a single key/description pair rendered inside a help section.
type helpRow struct {
	key  string
	desc string
}

// helpSection groups a titled set of rows. wideKeys uses a wider key column
// (15 chars instead of 12) — useful for sections whose keys are longer phrases.
type helpSection struct {
	title    string
	wideKeys bool
	rows     []helpRow
}

// helpContent holds all sections shown in the help overlay.
// To add, remove, or reorder entries, edit this slice only — no rendering code changes needed.
var helpContent = []helpSection{
	{
		title: "COLUMNS",
		rows: []helpRow{
			{"SYSCALL", "name of the kernel function called by the process"},
			{"FILE", "top observed file path for this syscall (truncated)"},
			{"CAT", "category: I/O · FS · NET · MEM · PROC · SIG · OTHER"},
			{"CALLS", "total number of times this syscall was called"},
			{"FREQ", "bar showing count relative to the most-called syscall"},
			{"AVG", "average time the kernel spent executing this syscall (yellow = slow ≥5ms)"},
			{"MAX", "peak (worst) latency — outliers that avg hides"},
			{"TOTAL", "cumulative CPU time spent inside this syscall"},
			{"ERRORS", "number of calls that returned an error (red)"},
			{"ERR%", "percentage of calls that returned an error (red)"},
		},
	},
	{
		title: "ROW COLOURS",
		rows: []helpRow{
			{"white", "normal — no issues detected"},
			{"yellow", "slow — AVG latency ≥ 5ms (kernel spending time here)"},
			{"orange", "some errors, but ERR% < 50% (often harmless)"},
			{"red", "critical — more than half of all calls are failing"},
		},
	},
	{
		title: "CATEGORY BAR",
		rows: []helpRow{
			{"I/O", "read, write, openat, close — file descriptor operations"},
			{"FS", "stat, fstat, access, lseek — filesystem metadata"},
			{"NET", "socket, connect, send, recv, epoll — networking"},
			{"MEM", "mmap, munmap, mprotect, madvise — memory management"},
			{"PROC", "clone, execve, wait, prctl — process/thread control"},
			{"SIG", "rt_sigaction, sigprocmask — signal handling"},
		},
	},
	{
		title:    "COMMON PATTERNS",
		wideKeys: true,
		rows: []helpRow{
			{"openat ERR%", "dynamic linker searches multiple paths — usually harmless"},
			{"recvfrom ERR%", "EAGAIN on non-blocking socket — normal for async I/O"},
			{"connect ERR%", "Happy Eyeballs: IPv4 and IPv6 tried in parallel, loser fails"},
			{"ioctl ERR%", "process has no TTY (running under sudo or piped)"},
			{"madvise ERR%", "memory hints rejected by kernel — informational"},
			{"high I/O%", "process is doing heavy file or socket data transfer"},
			{"high FS%", "process is scanning directories or checking many files"},
			{"high SIG%", "many signal handlers registered — common during lib init"},
		},
	},
	{
		title: "KEYBOARD SHORTCUTS",
		rows: []helpRow{
			{"↑ / k", "move selection up"},
			{"↓ / j", "move selection down"},
			{"enter / d", "open detail page for selected syscall"},
			{"c", "sort by COUNT (most called first)"},
			{"t", "sort by TOTAL time (most CPU in kernel)"},
			{"a", "sort by AVG latency"},
			{"x", "sort by MAX latency (find worst outlier)"},
			{"e", "sort by error count"},
			{"n", "sort alphabetically"},
			{"g", "group by category (I/O, FS, NET, MEM, PROC, SIG, OTHER)"},
			{"/", "filter: type a syscall name to narrow the list"},
			{"esc", "clear filter / deselect"},
			{"?", "this help screen"},
			{"q / Ctrl+C", "quit"},
		},
	},
}

// RenderHelp returns the help overlay content as a string, formatted to fit the given width.
// The help overlay provides explanations of the columns, row colours, category bar, common
// patterns, and keyboard shortcuts.
func RenderHelp(w int) string {
	var sb strings.Builder

	div := styles.DivStyle.Render(strings.Repeat("─", w))

	sb.WriteString(styles.TitleStyle.Width(w).Render(" stracectl — help  (press any key to close) ") + "\n")
	sb.WriteString(div + "\n")

	for _, sec := range helpContent {
		sb.WriteString("\n")
		sb.WriteString(styles.HeaderStyle.Render(" "+sec.title) + "\n")
		sb.WriteString(div + "\n")

		keyWidth := 12
		if sec.wideKeys {
			keyWidth = 15
		}

		for _, r := range sec.rows {
			sb.WriteString(styles.ActiveSortStyle.Render(fmt.Sprintf("  %-*s", keyWidth, r.key)))
			sb.WriteString(styles.RowStyle.Render(r.desc) + "\n")
		}
	}

	sb.WriteString("\n")
	sb.WriteString(styles.FooterStyle.Render(" press any key to return "))

	return sb.String()
}
