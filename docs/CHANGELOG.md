# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

---

## [Unreleased]

---

## [1.0.22] — 2026-03-08

### Added

#### `stracectl stats <file>` — Post-Mortem Analysis

New `stats` subcommand that reads a raw strace output file and displays the same
aggregated TUI or HTTP API as a live trace session — without re-running the process.

The file must have been captured with `strace -T` for latency data:

```bash
strace -T -o trace.log <command>
stracectl stats trace.log
```

Supports all the same modes as `run` and `attach`:

| Flag | Effect |
| ---- | ------ |
| _(none)_ | Opens the interactive TUI |
| `--serve :8080` | Exposes the HTTP API (JSON, WebSocket, Prometheus, web dashboard) |
| `--report <path>` | Writes a self-contained HTML report after the TUI exits |

---

#### `--report <path>` Flag on `run`, `attach`, and `stats`

All trace commands now accept a `--report <path>` flag. When set, a self-contained
HTML file is written when the session ends (on clean exit, SIGINT, or process end).

The report includes:

- **Header** — command or file traced, generation timestamp, total duration
- **Summary bar** — total syscalls, unique syscall count, overall error rate
- **Category breakdown** — bar chart of I/O / FS / NET / MEM / PROC / SIG / OTHER
- **Syscall table** — all columns (NAME, CAT, COUNT, FREQ %, AVG, MIN, MAX, TOTAL,
  ERR%) with sortable column headers (plain JavaScript, no external dependencies)

The file is fully self-contained (no CDN links) — safe for air-gapped environments
and suitable for attaching to incident reports or archiving.

---

#### Kubernetes Sidecar — Hardened `securityContext`

The sidecar manifest (`deploy/k8s/sidecar-pod.yaml`) and Helm chart
(`deploy/helm/stracectl/values.yaml`, `_helpers.tpl`) now apply a tighter
security context:

```yaml
securityContext:
  runAsUser: 0
  allowPrivilegeEscalation: false
  readOnlyRootFilesystem: true
  capabilities:
    drop: [ALL]
    add: [SYS_PTRACE]
```

This limits the blast radius to the minimum capability needed while keeping
`ptrace` functional.

---

#### Test Coverage Enforcement ≥ 80 %

Both the lefthook pre-push hook and the GitHub Actions CI workflow now enforce a
minimum 80 % statement coverage across all packages. Any push or PR that drops
coverage below this threshold fails immediately with a clear error message showing
the actual total coverage percentage.

---

### Added

#### Per-Syscall Detail Page (`GET /syscall/{name}`)

Each row in the web dashboard is now clickable. Clicking a syscall name navigates to
a dedicated detail page at `/syscall/<name>` that provides two main sections:

- **Live stat cards** — 7 cards (Calls, Avg / Min / Max Latency, Total Time, Errors,
  Error Rate) updated every second via the existing WebSocket stream. No additional
  connection needed.
- **Reference panel** — static inline documentation rendered immediately, without
  any external request. Covers the syscall signature, argument descriptions, return
  value meaning, error notes, and general usage notes for ~80 well-known Linux
  syscalls. Unknown syscalls receive a generic fallback with a `man 2 <name>` hint.

A **← Dashboard** back-link and a colour-coded category pill appear in the page header,
keeping visual consistency with the main dashboard.

---

#### `GET /api/syscall/{name}` — Single-Syscall JSON Endpoint

Returns the `SyscallStat` object for one syscall as JSON.  
Returns `404 Not Found` if the syscall has not yet been observed in the current trace.  
Used internally by the detail page and available for external tooling (scripts, custom
dashboards, alerting pipelines).

---

#### Aggregator — `Get(name string)` Method

New thread-safe point-lookup method on `Aggregator`.  
Acquires a read lock, copies the target `SyscallStat` by value, and returns it.  
This avoids exposing the internal pointer and keeps concurrent reads safe.

---

#### Sort by Category (`g` key) in the TUI

Pressing `g` in the TUI groups rows by syscall category in a fixed order:
**I/O → FS → NET → MEM → PROC → SIG → OTHER**, with rows sorted by call count within
each group. A second press restores the default (count) sort.

---

#### Live HTML Dashboard at `GET /`

The root URL now serves a single-page application instead of returning 404.  
Features:
- Table of all observed syscalls with category pills, spark bars, and colour-coded
  cells for high error rates and high latencies.
- Column header clicks to sort (calls, avg latency, errors, category).
- Auto-reconnects to the WebSocket stream if the server is restarted.
- No build step or external dependencies — served as an embedded Go string constant.

---

### Changed

#### TUI Redesign

The terminal UI received a full layout overhaul:

| What changed | Before | After |
|---|---|---|
| Header | Two separate bars (title + counters) | Single merged title bar |
| Counters position | Own row | Right side of the title bar |
| Column `REQ` | Showed raw request count | Renamed to `CALLS` |
| New columns | — | `ERRORS` and `ERR%` added |
| Category bar | Pipe-separated labels | Space-separated colour-coded pills |
| Alerts panel | Above the table | Below the table, above the footer |
| Divider colour | 238 (may be invisible on some terminals) | 241 (always visible) |
| New divider | — | Between title bar and category pills row |
| `fixedLines` | 6 (then 7) | 8 (matches the actual number of fixed rows) |

#### Aggregator — `MinTime` Tracking

The aggregator now records the minimum observed duration for each syscall.  
`MinTime` is surfaced through `Aggregator.Get()` and `/api/syscall/{name}`.  
The bulk `/api/stats` WebSocket stream does not yet include `MinTime` (listed under
Known Limitations in README).

#### License

Changed from MIT to **Apache 2.0**.

---

### Fixed

#### Parser — Hex Return Values Silently Dropped

Syscalls that return a memory address (e.g. `mmap` returning `0x7f3c…`) were
never recorded because the return-value regex only matched decimal integers.  
The `retRe` pattern now matches both decimal and `0x…` hexadecimal return values.

---

#### TUI Cursor Invisible When Scrolled Past Viewport

Selecting a row beyond the visible terminal height caused the highlighted row to be
rendered outside the screen. A `scrollOffset` variable is now maintained so the
selected row is always within the visible range.

---

#### Noisy Error Log on Clean Shutdown

When the traced process exited normally via `SIGTERM`, the tracer logged it as an
error. The shutdown path now checks `ExitCode() == -1` (the value `os/exec` sets for
signal termination) and skips the error log in that case.

---

#### `writeJSON` — Superfluous `WriteHeader` on Encode Error

`http.Error` was called after the response headers had already been written by a
successful `json.Encode`, triggering a `"superfluous response.WriteHeader"` warning
in the Go HTTP server and sending a corrupted response body.  
Fixed by encoding JSON into a buffer first, then writing headers and body only when
encoding succeeds.

---

#### Web Dashboard — Root Path Returning 404

`http.ServeMux` did not match the exact path `/` when other patterns were registered.
Added a dedicated `handleDashboard` handler with an explicit `r.URL.Path != "/"`
guard that returns a 404 for any sub-path that is not handled by another route.

---

#### TUI — Blue Syscall Names Masking Row Colours

Syscall names were styled with a blue `nameStyle` that overrode the row-level colour
(yellow for high latency, red for high error rate), making those visual cues invisible.  
Removed `nameStyle`; row-level colours now apply to the full row including the name.

---

#### TUI Header Duplicated on First Render

`fixedLines` tracked the number of static header rows used to compute the scrollable
area. After adding layout elements (category pills, a second divider, the title-bar
divider) without updating the count, the header was duplicated on the very first
render frame. Corrected to the current value of 8.

---

#### Web Detail Page Stuck at "Connecting…" (TDZ Bug)

`renderRef(NAME)` was called before `const esc` was declared in the inline script.
Because `const` declarations are not hoisted (unlike `function`), this triggered a
`ReferenceError` in the temporal dead zone that silently aborted the entire `<script>`
block — including the `connect()` call at the end — leaving the page stuck at the
"Connecting…" status message.  
Fixed by moving the `renderRef(NAME)` invocation to after all `const` helper
definitions.

---

## [0.1.0] — 2025

- Initial `stracectl` TUI: real-time syscall aggregation via `strace`, BubbleTea
  interface with per-syscall counts, latencies, error rates, and category breakdown.
- `stracectl run <cmd>` — trace a command from the start.
- `stracectl attach <pid>` — attach to a running process.
- `stracectl discover <container-name>` — find the PID of a container inside a
  shared-PID-namespace Pod by inspecting `/proc/<pid>/cgroup`.
- Sidecar mode (`--serve <addr>`) with JSON (`/api/stats`, `/api/categories`),
  WebSocket (`/stream`), and Prometheus (`/metrics`) endpoints.
- Kubernetes deployment manifests (`deploy/k8s/`) and Helm chart
  (`deploy/helm/stracectl/`).
- Dockerfile and docker-compose for local development with hot-reload support.
- GitHub Actions: CI (vet + lint + test), Docker image build and push, Trivy
  vulnerability scanning, and binary release workflow.
- `golangci-lint` v2 configuration; pinned to a stable version in CI.
- Go upgraded to 1.26.1 to resolve 19 stdlib security vulnerabilities.
- Apache 2.0 license.
