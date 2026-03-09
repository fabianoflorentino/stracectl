---
title: "Changelog"
description: "All notable changes to stracectl."
---

All notable changes to stracectl are documented here.
The format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

---

## v1.0.23 — 2026-03-09

### Added

**Per-Errno Error Breakdown** — The aggregator now records a breakdown of errors by errno code (`ErrorBreakdown map[string]int64`). The most-frequent errno codes are surfaced in the web detail page, making it easy to differentiate `ENOENT`, `EACCES`, `EAGAIN`, and others without inspecting raw strace output.

**Recent Error Samples Ring Buffer** — A bounded ring buffer (`maxErrorSamples = 50`) captures the most recent failed calls, each with a timestamp, errno string, and raw args. Accessible via `/api/syscall/{name}`.

**Anomaly Alerts Panel (Web UI)** — The web dashboard now shows an anomaly panel whenever any syscall crosses a threshold: ≥ 50% error rate (red) or AVG latency ≥ 5 ms (yellow). Each alert includes a plain-English explanation. The panel is hidden when there are no active anomalies.

**P95 / P99 Latency Percentiles** — The aggregator maintains a log₂ histogram (`latHist [63]int64`) per syscall, enabling O(1) approximate percentile computation. P95 and P99 latencies are exposed through `/api/syscall/{name}` and shown on the web detail page.

**Process Metadata from `/proc`** — `/api/status` now includes full process metadata: executable path (`Exe`), full command line (`Cmdline`), and current working directory (`Cwd`). The web dashboard header displays the resolved command being traced.

**Sliding Window Error Rate (60s)** — The aggregator tracks per-second error counts in a 60-bucket rolling window. `ErrRate60s` is updated atomically on every event and shown in the web dashboard header.

**Live Log Tab (Web UI)** — A **LIVE LOG** tab in the web dashboard streams the most recent 500 syscall events from a new `/api/log` endpoint, polled every second. The ring buffer is capped at 500 entries.

**Syscall Search / Filter (Web UI)** — A filter bar above the syscall stats table narrows rows in real time (client-side). A clear (✕) button resets the filter; a match counter shows how many syscalls satisfy the current query.

**Process Exited Notification (Web UI)** — An amber banner appears when the traced process exits: *"⏹ Process exited — trace complete. Data frozen."*

### Fixed

**TUI Column Misalignment on Multibyte Characters** — Column padding helpers used `len(s)` (byte count) instead of display width. Characters like `µ` and `—` caused column shift. Fixed by replacing `len(s)` with `lipgloss.Width(s)` throughout all padding helpers.

---

## v1.0.22 and earlier

See the [full CHANGELOG on GitHub](https://github.com/fabianoflorentino/stracectl/blob/main/docs/CHANGELOG.md).
