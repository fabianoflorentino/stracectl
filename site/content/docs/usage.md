---
title: "Usage"
description: "How to run, attach, and analyse traces with stracectl."
weight: 2
---

## Trace a command from the start

### Global flags

These flags are available to all commands (place before the subcommand):

- `--ws-token <token>` — require a Bearer token for WebSocket connections when using `--serve`.
- `--debug` — enable verbose tracer diagnostics. When set, `stracectl` will emit
	raw strace lines useful for diagnosing parser edge cases (use only for troubleshooting).


```bash
sudo stracectl run curl https://example.com
sudo stracectl run -- python3 app.py --port 8080
```

Save a self-contained HTML report when the session ends:

```bash
sudo stracectl run --report report.html curl https://example.com
```

## Backend selection

Choose the tracing backend with `--backend` (values: `auto`, `ebpf`, `strace`).
`auto` picks the eBPF backend when the binary was built with eBPF support and
the running kernel supports the required features (Linux >= 5.8); otherwise it
falls back to the classic `strace` subprocess tracer. Use `--backend ebpf` to
force the eBPF backend or `--backend strace` to force the subprocess tracer.

See the dedicated page for more details: [eBPF backend](/docs/ebpf/).

## Attach to a running process

```bash
sudo stracectl attach 1234
sudo stracectl attach "$(pgrep nginx | head -1)"
```

Attach with an HTML report on exit:

```bash
sudo stracectl attach --report nginx-report.html 1234
```

## Post-mortem analysis

If you already have a log captured with `strace -T`, load it into
the same TUI without needing a live process:

```bash
# Capture
strace -T -o trace.log curl https://example.com

# TUI
stracectl stats trace.log

# HTTP API  
stracectl stats --serve :8080 trace.log

# HTML report
stracectl stats --report report.html trace.log
```

## Keyboard shortcuts

| Key | Action |
|-----|--------|
| `↑` / `k` | Move cursor up |
| `↓` / `j` | Move cursor down |
| `Enter` / `d` | Open detail overlay |
| `c` | Sort by CALLS (default) |
| `t` | Sort by TOTAL time |
| `a` | Sort by AVG latency |
| `x` | Sort by MAX latency |
| `e` | Sort by error count |
| `n` | Sort alphabetically |
| `g` | Sort by category |
| `/` | Open filter prompt |
| `Esc` | Clear filter / reset cursor |
| `?` | Open help overlay |
| `q` / `Ctrl+C` | Quit |

## Reading the dashboard

### Header bar

```
stracectl  /usr/local/bin/curl  +4s   syscalls: 472  rate: 118/s  errors: 35  unique: 40
```

- **target** — path or command being traced
- **elapsed** — wall-clock time since tracing started
- **syscalls** — total calls captured
- **rate** — current syscalls/second
- **errors** — absolute count of failed calls
- **unique** — number of distinct syscall names

### Category bar

```
I/O 35%   FS 28%   NET 18%   MEM 9%   PROC 7%   OTHER 3%
```

Instant breakdown of activity by category. A spike in NET or FS often
points to the source of latency or errors.

### Anomaly highlights

- **Red row** — ERR% ≥ 50%
- **Yellow row** — AVG latency ≥ 5 ms
- **Orange row** — any errors present
- **Alert bar** — plain-English explanation of the most prominent anomaly

### Detail overlay

Press `Enter` or `d` on any row to open the detail overlay. It shows:

- Syscall reference (description, signature, arguments)
- Return values and common errno codes
- Live statistics (calls, avg/min/max/P95/P99 latency, total time, error rate)
