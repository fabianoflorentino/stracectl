---
title: "HTTP API"
description: "Reference for the stracectl HTTP, WebSocket, and Prometheus API."
weight: 4
---

## Enabling sidecar / HTTP mode

Pass `--serve <addr>` to any command to replace the TUI with an HTTP server:

```bash
sudo stracectl run   --serve :8080 curl https://example.com
sudo stracectl attach --serve :8080 1234
stracectl stats      --serve :8080 trace.log
```

## Endpoints

### `GET /`

Live HTML dashboard. Auto-polls every second. Displays:

- Per-syscall stats table with clickable rows
- Anomaly alert panel
- Process metadata header
- Live log tab

### `GET /api/syscalls`

Returns a JSON array of all aggregated syscall records:

```json
[
  {
    "name": "openat",
    "category": "I/O",
    "count": 77,
    "errors": 18,
    "avg_ns": 36800,
    "max_ns": 2800000,
    "total_ns": 2836600,
    "p95_ns": 95000,
    "p99_ns": 1200000,
    "err_rate": 0.234
  }
]
```

### `GET /api/syscall/{name}`

Detailed record for one syscall, including:
- All latency stats (avg, min, max, P95, P99, total)
- `error_breakdown` — map of errno string → count
- `recent_errors` — last 50 failed calls with timestamp, errno, and args

### `GET /api/status`

Global counters and process metadata:

```json
{
  "pid": 1234,
  "exe": "/usr/bin/curl",
  "cmdline": ["curl", "https://example.com"],
  "cwd": "/home/user",
  "elapsed_ms": 4200,
  "total_syscalls": 472,
  "total_errors": 35,
  "rate_per_sec": 118,
  "unique_syscalls": 40,
  "err_rate_60s": 0.074,
  "exited": false
}
```

### `GET /api/log`

Returns the most recent 500 syscall events as a JSON array (newest last).

### `WebSocket /ws`

Real-time stream of `SyscallEvent` objects:

```json
{
  "pid": 1234,
  "name": "openat",
  "args": "AT_FDCWD, \"/etc/ld.so.conf\", O_RDONLY",
  "ret": "3",
  "latency_ns": 38200,
  "error": "",
  "timestamp": "2026-03-09T12:34:56.789Z"
}
```

### `GET /metrics`

Prometheus text format. Exposes per-syscall counters, error rates, and latency histograms. Compatible with Prometheus Operator `ServiceMonitor`.

## Web detail page

Navigate to `/syscall/<name>` (or click any row in the dashboard) for:

- 9 stat cards: calls, avg/min/max/P95/P99 latency, total time, errors, error rate
- Errno breakdown chart
- Recent error samples ring buffer (last 50 failures)
- Full syscall reference panel (~80 well-known Linux syscalls)
