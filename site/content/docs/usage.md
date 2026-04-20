---
title: "Usage"
description: "How to run, attach, and analyse traces with stracectl."
weight: 3
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

## Per-PID aggregation

Use `--per-pid` to split syscall rows by process ID instead of combining all
events into one row per syscall name.

```bash
# live trace
sudo stracectl run --per-pid -- python3 app.py

# attach mode
sudo stracectl attach --per-pid 1234

# post-mortem analysis
stracectl stats --per-pid trace.log
```

When enabled, the TUI table shows a `PID` column (replacing `FILE`).

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

## Per-file view

`stracectl` can surface the most-opened file paths observed during a trace. Use
the TUI overlay (press `f`) to inspect hot files interactively, or query the
sidecar API:

```bash
curl -s 'http://localhost:8080/api/files?limit=20' | jq .
```

Include the Top Files table in exported HTML reports with `--report-top-files N`.

## Local usage and security

For short, ephemeral troubleshooting sessions prefer running the HTTP sidecar bound to `127.0.0.1` or using `kubectl port-forward` when inspecting a sidecar in a cluster. This minimizes accidental exposure of the WebSocket (`/stream`), JSON API, and `/metrics`.

- Bind to localhost:

```bash
sudo stracectl run --serve 127.0.0.1:8080 <command>
```

- Port-forward a sidecar from the cluster:

```bash
kubectl -n <ns> port-forward pod/<sidecar-pod> 8080:8080
```

- If you must expose the server beyond localhost (for monitoring or long-term use), require `--ws-token` and TLS (ingress or proxy). Prefer presenting the token via the `Authorization: Bearer <token>` header instead of query strings, and avoid passing secrets in URLs.

- Protect `/metrics` by limiting Prometheus scrape to internal networks or requiring authentication.

These measures keep troubleshooting convenient while reducing the risk of accidental public exposure.

## Privacy flags

Use the following flags to control privacy-sensitive behaviour. These flags are designed to default to safe behavior and provide explicit, auditable options when more data is required.

- `--privacy-log <path|stdout>` — write newline-delimited redacted JSON events to a file or `stdout`. When writing to a file, an audit file `<path>.audit` is created alongside it with `trace_start`/`trace_end` metadata and a SHA256 of the trace file.
- `--privacy-ttl <duration>` — optional TTL to automatically expire ephemeral privacy logs (examples: `24h`, `15m`). Best-effort overwrite is attempted before deletion; prefer encrypted volumes or tmpfs for stronger guarantees.
- `--no-args` — suppress capture of syscall argument content entirely (maximal privacy).
- `--max-arg-size N` — when arguments are captured, truncate each to at most `N` bytes (default: 64).
- `--redact-patterns=pat1,pat2` — add custom regex patterns to the redaction set.
- `--privacy-level low|medium|high` — pre-baked privacy preset (default: `high`). `low` enables more verbose captures; `high` limits to metadata and aggressive redaction.
- `--full` — enable full payload capture (dangerous). `--force` is required in non-interactive contexts to proceed with `--full`.

Quick examples:

```bash
# redacted file output that expires in 24 hours
stracectl run --privacy-log trace.json --privacy-ttl 24h --no-args curl https://example.com

# stream redacted events to stdout (no audit file created)
stracectl run --privacy-log stdout --no-args my-command

# explicit full capture (use only with authorization)
stracectl run --privacy-log trace-full.json --full --force my-command
```

See also: [Privacy page](/docs/privacy/) and `docs/privacy-usage-examples.md` for more guidance.

## Explain current privacy settings

Use the `explain` subcommand to preview what will be captured under the current
privacy settings without running a trace. This is useful to verify redaction
behavior before committing to a full session:

```bash
stracectl explain
stracectl explain --no-args --redact-patterns="token,secret"
stracectl explain --privacy-level low
```

The command prints:

- All active privacy option values
- Compiled redaction patterns
- An example of a redacted event showing what the output will look like

Run `stracectl explain` before any sensitive trace session to confirm that no
unintended data will be captured or logged.

## Keyboard shortcuts

| Key | Action |
| ----- | -------- |
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

```shell
stracectl  /usr/local/bin/curl  +4s   syscalls: 472  rate: 118/s  errors: 35  unique: 40
```

- **target** — path or command being traced
- **elapsed** — wall-clock time since tracing started
- **syscalls** — total calls captured
- **rate** — current syscalls/second
- **errors** — absolute count of failed calls
- **unique** — number of distinct syscall names

### Category bar

```shell
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
