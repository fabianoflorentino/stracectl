# stracectl

A modern `strace` with a real-time, htop-style TUI.

Instead of scrolling through a wall of syscall output, `stracectl` aggregates
everything live and presents it in an interactive dashboard: per-syscall counts,
latencies, error rates, and category breakdown — all updated while the process runs.

```text
 stracectl  curl google.com                   elapsed: 4s
  syscalls: 472    rate:  892/s   unique: 40   errors: 35 (7.4%)
  I/O:35%  │  FS:28%  │  NET:18%  │  MEM:9%  │  PROC:7%  │  OTHER:3%
  Process is mainly reading and writing data (60%) — ✓ 7% errors (likely normal)
──────────────────────────────────────────────────────────────────────────────────
SYSCALL        CAT      REQ  FREQ              AVG      MAX      TOTAL    ERR%
──────────────────────────────────────────────────────────────────────────────────
⚠ connect: 45% error rate (5/11 calls) — Happy Eyeballs: IPv4/IPv6 race, loser fails
──────────────────────────────────────────────────────────────────────────────────
openat         I/O       77  ████████░░░░    36.8µs   2.8ms    2.8ms     23%
close          I/O       67  ███████░░░░░    31.9µs   595µs    2.1ms      —
fstat          FS        62  ██████░░░░░░    33.9µs   628µs    2.1ms      —
read           I/O       56  █████░░░░░░░    37.1µs   2.1ms    2.1ms      —
connect        NET        6  █░░░░░░░░░░░    41.3µs   248µs    248µs     50%
──────────────────────────────────────────────────────────────────────────────────
 q:quit  c:count▼  t:total  a:avg  x:max  e:errors  n:name  /:filter  ?:help
```

## Features

- **Real-time aggregation** — syscalls counted, timed, and grouped as they happen; no log file needed
- **Latency columns** — AVG, MAX, and TOTAL time spent in kernel; MAX exposes outliers that averages hide
- **ERR%** — error rate per syscall; `access` at 100% (2/2) is more alarming than `openat` at 23% (18/77)
- **Category bar** — instant overview: I/O · FS · NET · MEM · PROC · SIG · OTHER
- **Summary line** — plain-English sentence describing what the process is doing and its health
- **FREQ sparkbar** — visual proportion of each syscall relative to the most-called one
- **Live rate** — syscalls/second, recalculated every 500 ms
- **Anomaly highlighting** — rows turn yellow when AVG ≥ 5 ms, red when ERR% ≥ 50%
- **Smart alerts** — banner with human-readable explanation of why the error is happening
- **Interactive filter** — press `/` and type to narrow down syscalls in real time
- **Help overlay** — press `?` for a full in-app reference of every column, colour, and pattern
- **Multiple sort keys** — count, total time, avg latency, peak latency, errors, name

## Requirements

- Linux (uses `ptrace` via the `strace` binary)
- Go 1.21+
- `strace` installed

```bash
# Debian / Ubuntu
sudo apt install strace

# Fedora / RHEL
sudo dnf install strace
```

## Install

```bash
git clone https://github.com/fabianoflorentino/stracectl
cd stracectl
go build -o stracectl .
sudo mv stracectl /usr/local/bin/
```

## Usage

### Trace a command from the start

```bash
sudo stracectl run curl https://example.com
sudo stracectl run -- python3 app.py --port 8080
```

### Attach to a running process

```bash
sudo stracectl attach 1234
sudo stracectl attach "$(pgrep nginx | head -1)"
```

> **Permissions:** `strace` requires `CAP_SYS_PTRACE`.
> Run with `sudo`, or set `/proc/sys/kernel/yama/ptrace_scope` to `0` for your user.

## Keyboard shortcuts

| Key | Action |
| ----- | -------- |
| `c` | sort by COUNT (default) |
| `t` | sort by TOTAL time |
| `a` | sort by AVG latency |
| `x` | sort by MAX latency |
| `e` | sort by error count |
| `n` | sort alphabetically |
| `/` | open filter prompt |
| `esc` | clear filter |
| `?` | open help overlay |
| `q` / `Ctrl+C` | quit |

## Reading the dashboard

### Stats bar

```text
syscalls: 472    rate: 892/s   unique: 40   errors: 35 (7.4%)
```

- **syscalls** — total calls captured since tracing started
- **rate** — current syscalls/second; a sudden spike or drop is the first sign of anomaly
- **unique** — number of distinct syscall names; low value on a busy process often means a tight loop
- **errors** — absolute count and percentage of failed calls

### Category bar

```text
I/O:35%  │  FS:28%  │  NET:18%  │  MEM:9%  │  PROC:7%  │  OTHER:3%
```

Tells you at a glance what the process is doing.
A server idling should show mostly NET.
A process at 80%+ FS is scanning directories or checking many files.

| Category | Syscalls included |
| -------- | ---------------- |
| I/O | `read`, `write`, `openat`, `close`, `pread64`, … |
| FS | `stat`, `fstat`, `access`, `lseek`, `getdents64`, … |
| NET | `socket`, `connect`, `sendto`, `recvfrom`, `epoll_wait`, … |
| MEM | `mmap`, `munmap`, `mprotect`, `madvise`, `brk`, … |
| PROC | `clone`, `execve`, `wait4`, `prctl`, `getpid`, … |
| SIG | `rt_sigaction`, `rt_sigprocmask`, `eventfd`, … |
| OTHER | everything not in the above categories |

### Summary line

```text
Process is mainly reading and writing data (60%), then networking (11%) — ✓ 7% errors (likely normal)
```

A plain-English sentence that combines the dominant category with a health indicator:

| Indicator | Meaning |
| --------- | ------- |
| `✓ no errors` | all syscalls succeeded |
| `✓ X% (likely normal)` | errors below 15% — usually harmless (linker searches, EAGAIN) |
| `⚠ X% (worth investigating)` | errors between 15–40% |
| `✗ X% (high, check alerts)` | errors above 40% |

### Row colours

| Colour | Meaning |
| -------- | ------- |
| White | normal |
| **Yellow** | AVG latency ≥ 5 ms — kernel spending significant time here |
| Orange | some errors, ERR% < 50% — often harmless |
| **Red bold** | ERR% ≥ 50% — more than half of all calls are failing |

### Anomaly alerts

When a row crosses a threshold, a banner with an explanation appears above the data:

```text
⚠  ioctl: 100% error rate (3/3 calls) — terminal control failed (no TTY)
⚠  connect: 45% error rate — Happy Eyeballs: IPv4/IPv6 tried in parallel, loser fails
⚡  openat: slow avg 8.2ms (max 34ms) — kernel spending time in this call
```

### Common patterns explained

| What you see | Why it happens | Is it a problem? |
| --- | --- | --- |
| `openat` high ERR% | dynamic linker searches many paths before finding the `.so` | No |
| `recvfrom` high ERR% | `EAGAIN` on a non-blocking socket — no data ready yet | No |
| `connect` ~50% ERR% | Happy Eyeballs: IPv4 and IPv6 raced, loser is discarded | No |
| `ioctl` 100% ERR% | process has no TTY (running piped or under `sudo`) | No |
| `madvise` ERR% | kernel rejected memory hint — informational | No |
| `access` 100% ERR% | optional config file does not exist | Rarely |
| any syscall yellow | slow kernel path — I/O wait, lock contention, or disk | Investigate |
| any syscall red | repeated real failures | Yes |

### Help overlay

Press `?` at any time to open a full in-app reference covering every column,
colour, category, common pattern, and keyboard shortcut. Press any key to return.

## Project structure

```text
stracectl/
├── main.go
├── cmd/
│   ├── root.go              # Cobra root command
│   ├── attach.go            # stracectl attach <pid>
│   └── run.go               # stracectl run <cmd>
└── internal/
    ├── models/
    │   └── event.go         # SyscallEvent struct
    ├── parser/
    │   └── parser.go        # parses strace output lines → SyscallEvent
    ├── aggregator/
    │   └── aggregator.go    # thread-safe stats, categories, sorting
    ├── tracer/
    │   └── strace.go        # spawns strace subprocess, emits events on a channel
    └── ui/
        ├── tui.go           # BubbleTea full-screen TUI
        └── syscall_help.go  # syscall descriptions and errno explanations
```

### Architecture

```text
strace (subprocess)
    │  stderr — one line per syscall
    ▼
parser.Parse()
    │  chan SyscallEvent  (buffered 4096)
    ▼
aggregator.Add()     ← dedicated goroutine, mutex-protected
    │
    └─► ui.Run()     ← BubbleTea, redraws every 200 ms
```

## Running the tests

```bash
# all packages
go test ./internal/...

# with race detector (recommended)
go test ./internal/... -race

# verbose output
go test ./internal/... -v
```

## Roadmap

- [ ] Direct `ptrace` backend (remove dependency on the `strace` binary)
- [ ] eBPF backend via `cilium/ebpf` (zero overhead, suitable for production)
- [ ] Per-file view — which paths are opened most often
- [ ] Per-socket view — connections, bytes sent and received
- [ ] Flamegraph-style syscall timeline
- [ ] `stracectl stats <file>` — post-mortem analysis of a saved trace
- [ ] Process tree view for multi-process tracing (`-f`)

## Dependencies

| Package | Purpose |
| -------- | ------- |
| [charmbracelet/bubbletea](https://github.com/charmbracelet/bubbletea) | TUI framework |
| [charmbracelet/lipgloss](https://github.com/charmbracelet/lipgloss) | terminal styling |
| [spf13/cobra](https://github.com/spf13/cobra) | CLI commands |

## License

MIT
