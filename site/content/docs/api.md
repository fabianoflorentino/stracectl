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

All endpoints are served from the embedded HTTP server when using `--serve` or
`attach --serve`. The primary API endpoints are listed below.

### `GET /`

Live HTML dashboard (single-page app). Auto-polls every second. Displays:

- Per-syscall stats table with clickable rows
- Anomaly alert panel
- Process metadata header
- Live log tab

### `GET /api`

Lists available API endpoints exposed by the running server. Supports pagination
via the query parameters `page` (1-based) and `per_page`.

Response schema:

```json
{
  "total": 13,
  "page": 1,
  "per_page": 20,
  "items": [
    { "path": "/api/status", "method": "GET", "description": "Current trace/status information" },
    { "path": "/api/stats", "method": "GET", "description": "Aggregated syscall statistics" }
  ]
}
```

Example:

```bash
curl -s 'http://localhost:8080/api?page=1&per_page=20' | jq
```

### `GET /api/stats`

Returns a JSON array of aggregated syscall statistics (sorted by frequency). Each
item contains fields used by the dashboard SPA:

- `Name` (string)
- `Category` (string, e.g. "I/O", "FS")
- `Count` (int)
- `Errors` (int)
- `TotalTime`, `MinTime`, `MaxTime`, `P95`, `P99` (integers in nanoseconds)
- `ErrRate60s` (int)
- `ErrorBreakdown` (map of errno → count, present only if errors occurred)

Example (abridged):

```json
[
  {
    "Name": "read",
    "Category": "I/O",
    "Count": 42,
    "Errors": 0,
    "TotalTime": 12345678,
    "P95": 50000
  }
]
```

### `GET /api/syscall/{name}`

Detailed record for a single syscall. Includes all latency stats, the
`ErrorBreakdown` map and `RecentErrors` samples (if any).

### `GET /api/status`

Global counters and process metadata. The server returns an object with the
following keys:

- `Proc` — process metadata (see `procinfo.ProcInfo`): `PID`, `Comm`, `Cmdline`, `Exe`, `Cwd`
- `Total` — total syscall count (int)
- `Errors` — total errors (int)
- `Rate` — recent rate (float, syscalls/second)
- `Unique` — number of unique syscall names observed (int)
- `Elapsed` — human-friendly elapsed time string (e.g. "2m3s")
- `Done` — boolean indicating traced process exited

Example:

```json
{
  "Proc": { "PID": 1234, "Comm": "curl", "Cmdline": "/usr/bin/curl ..." },
  "Total": 472,
  "Errors": 35,
  "Rate": 118.0,
  "Unique": 40,
  "Elapsed": "4s",
  "Done": false
}
```

### `GET /api/log`

Returns the most recent raw events (up to 500 entries) as a JSON array. Each
entry contains `Time`, `PID`, `Name`, `Args`, `RetVal`, and `Error`.

### `GET /api/categories`

Returns a JSON object mapping category labels (e.g. "I/O") to per-category
aggregates (`Count`, `Errs`). Useful for building the category summary bar.

### `WebSocket /stream`

Upgrades to a WebSocket and emits a snapshot array of `SyscallStat` objects
every second (same schema as `GET /api/stats`). If a `wsToken` is configured,
the connection must be authorized using a `Bearer` token in the `Authorization`
header or the `token` query parameter.

### `GET /metrics`

Prometheus exposition format. Exposes per-syscall counters, histograms and
gauges such as `stracectl_syscall_calls_total`, `stracectl_syscall_errors_total`,
`stracectl_syscall_latency_seconds` (histogram), `stracectl_syscalls_per_second`,
and runtime gauges like `stracectl_ws_clients` and `stracectl_tracer_backlog`.

## Web detail page

Navigate to `/syscall/<name>` (or click any row in the dashboard) for:

- Stat cards: calls, avg/min/max/P95/P99 latency, total time, errors, error rate
- Errno breakdown chart
- Recent error samples ring buffer (last failures)
- Built-in syscall reference panel
