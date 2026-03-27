package overlays

import (
	"fmt"
	"strings"

	"github.com/fabianoflorentino/stracectl/internal/ui/styles"
)

// RenderHelp renders the help overlay for width w.
func RenderHelp(w int) string {
	var sb strings.Builder

	titleLine := styles.TitleStyle.Width(w).Render(" stracectl — help  (press any key to close) ")
	div := styles.DivStyle.Render(strings.Repeat("─", w))

	sb.WriteString(titleLine + "\n")
	sb.WriteString(div + "\n")

	section := func(title string) {
		sb.WriteString("\n")
		sb.WriteString(styles.HeaderStyle.Render(" "+title) + "\n")
		sb.WriteString(div + "\n")
	}
	row := func(key, desc string) {
		sb.WriteString(styles.ActiveSortStyle.Render(fmt.Sprintf("  %-12s", key)))
		sb.WriteString(styles.RowStyle.Render(desc) + "\n")
	}
	patternRow := func(key, desc string) {
		sb.WriteString(styles.ActiveSortStyle.Render(fmt.Sprintf("  %-15s", key)))
		sb.WriteString(styles.RowStyle.Render(desc) + "\n")
	}

	section("COLUMNS")
	row("SYSCALL", "name of the kernel function called by the process")
	row("FILE", "top observed file path for this syscall (truncated)")
	row("CAT", "category: I/O · FS · NET · MEM · PROC · SIG · OTHER")
	row("CALLS", "total number of times this syscall was called")
	row("FREQ", "bar showing count relative to the most-called syscall")
	row("AVG", "average time the kernel spent executing this syscall (yellow = slow ≥5ms)")
	row("MAX", "peak (worst) latency — outliers that avg hides")
	row("TOTAL", "cumulative CPU time spent inside this syscall")
	row("ERRORS", "number of calls that returned an error (red)")
	row("ERR%", "percentage of calls that returned an error (red)")

	section("ROW COLOURS")
	row("white", "normal — no issues detected")
	row("yellow", "slow — AVG latency ≥ 5ms (kernel spending time here)")
	row("orange", "some errors, but ERR% < 50% (often harmless)")
	row("red", "critical — more than half of all calls are failing")

	section("CATEGORY BAR")
	row("I/O", "read, write, openat, close — file descriptor operations")
	row("FS", "stat, fstat, access, lseek — filesystem metadata")
	row("NET", "socket, connect, send, recv, epoll — networking")
	row("MEM", "mmap, munmap, mprotect, madvise — memory management")
	row("PROC", "clone, execve, wait, prctl — process/thread control")
	row("SIG", "rt_sigaction, sigprocmask — signal handling")

	section("COMMON PATTERNS")
	patternRow("openat ERR%", "dynamic linker searches multiple paths — usually harmless")
	patternRow("recvfrom ERR%", "EAGAIN on non-blocking socket — normal for async I/O")
	patternRow("connect ERR%", "Happy Eyeballs: IPv4 and IPv6 tried in parallel, loser fails")
	patternRow("ioctl ERR%", "process has no TTY (running under sudo or piped)")
	patternRow("madvise ERR%", "memory hints rejected by kernel — informational")
	patternRow("high I/O%", "process is doing heavy file or socket data transfer")
	patternRow("high FS%", "process is scanning directories or checking many files")
	patternRow("high SIG%", "many signal handlers registered — common during lib init")

	section("KEYBOARD SHORTCUTS")
	row("↑ / k", "move selection up")
	row("↓ / j", "move selection down")
	row("enter / d", "open detail page for selected syscall")
	row("c", "sort by COUNT (most called first)")
	row("t", "sort by TOTAL time (most CPU in kernel)")
	row("a", "sort by AVG latency")
	row("x", "sort by MAX latency (find worst outlier)")
	row("e", "sort by error count")
	row("n", "sort alphabetically")
	row("g", "group by category (I/O, FS, NET, MEM, PROC, SIG, OTHER)")
	row("/", "filter: type a syscall name to narrow the list")
	row("esc", "clear filter / deselect")
	row("?", "this help screen")
	row("q / Ctrl+C", "quit")

	sb.WriteString("\n")
	sb.WriteString(styles.FooterStyle.Render(" press any key to return "))

	return sb.String()
}
