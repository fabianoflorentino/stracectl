# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

---

## [Unreleased]

### Added

- **Per-syscall detail page** (`GET /syscall/{name}`) in the web dashboard.
  Clicking any row in the main dashboard navigates to a dedicated page that shows:
  - 7 live stat cards (Calls, Avg / Min / Max Latency, Total Time, Errors, Error Rate)
    updated every second via the WebSocket stream.
  - A reference panel (SYSCALL REFERENCE, ARGUMENTS, RETURN VALUE, NOTES) rendered
    immediately from an embedded JS table covering ~80 well-known Linux syscalls.
    Unknown syscalls receive a generic fallback with a `man 2 <name>` hint.
  - A **← Dashboard** back button in the header.
  - Category pill with the same colour coding as the main dashboard.

- **Clickable rows** in the web dashboard — every `<tr>` now carries a `data-name`
  attribute and a row-click listener that navigates to `/syscall/<name>` via
  `encodeURIComponent`. Rows display `cursor: pointer`.

- **`GET /api/syscall/{name}`** endpoint — returns a single `SyscallStat` as JSON.
  Responds with `404 Not Found` if the syscall has not been observed in the current
  trace. Used internally by the detail page and available for external tooling.

- **`Aggregator.Get(name string)`** — new thread-safe method for point lookup of a
  single syscall stat by name.

- **Sort by category** (`g` key in TUI) — groups rows by category (I/O → FS → NET →
  MEM → PROC → SIG → OTHER), then by call count within each group.

- **Live HTML dashboard** at `GET /` — single-page app that connects via WebSocket,
  renders a sortable table with category pills, spark bars, and colour-coded error /
  latency cells. Auto-reconnects if the server restarts.

- **`Aggregator.SortByCategory`** sort field used by the `g` key and the category
  sort button in the web dashboard.

### Changed

- **TUI header** merged into a single title bar: target process + elapsed time on the
  left, live counters (syscalls, rate, errors, unique) on the right.
- TUI column `REQ` renamed to `CALLS`.  
- TUI added `ERRORS` and `ERR%` columns.
- TUI category bar changed to space-separated colour-coded pills (no pipe separators).
- TUI alerts panel moved **below** the syscall table, between the last data row and
  the footer divider.
- TUI dividers updated from colour 238 → 241 to remain visible on all terminals.
- TUI added a divider between the title bar and the category pills row.
- TUI `fixedLines` corrected to 8 to prevent the header from being duplicated on the
  first render frame.
- Aggregator now tracks `MinTime` per syscall and exposes it through
  `Aggregator.Get()` and the `/api/syscall/{name}` endpoint. The bulk `/api/stats`
  endpoint does not yet include `MinTime` (see Known Limitations).
- License changed from MIT to Apache 2.0.

### Fixed

- **Parser dropping hex return values** — syscall events where `strace` reported the
  return value as a hex address (e.g. `mmap` returning `0x7f…`) were silently
  discarded. The `retRe` regex now matches both decimal and `0x…` return values.

- **TUI cursor invisible beyond viewport** — scrolling past the terminal height caused
  the selected row to be painted outside the visible area. A `scrollOffset` is now
  applied so the cursor always stays on screen.

- **Noisy log on clean shutdown** — a process correctly terminated by `SIGTERM` was
  logged as an error. The shutdown path now checks `ExitCode() == -1` before logging.

- **`writeJSON` double-header panic** — `http.Error` was called after headers had
  already been written by a successful JSON encode path, producing a
  `"superfluous response.WriteHeader"` warning and a corrupted response. Fixed by
  buffering the JSON before writing headers.

- **Web dashboard returning 404** — no handler was registered for the exact path `/`
  when using `http.ServeMux`. Added `handleDashboard` with an explicit `r.URL.Path
  != "/"` guard.

- **TUI blue syscall names masking row colours** — syscall names were rendered in blue
  (`nameStyle`), which overrode the row-level intensity colouring (yellow for slow,
  red for high error rate). Removed `nameStyle`; row-level colours restored.

- **TUI header duplicated on first render** — `fixedLines` was 6, then 7 after adding
  a divider, then 8 after adding the title-bar divider; each off-by-one caused the
  header to appear twice on the very first frame.

- **Web detail page stuck at "Connecting…"** — `renderRef(NAME)` was called before
  `const esc` was declared. Because `const` (unlike `function`) is not hoisted, this
  threw a `ReferenceError` in the temporal dead zone that silently aborted the entire
  `<script>` block, including `connect()`. Fixed by moving `renderRef(NAME)` to after
  all `const` helper definitions.

---

## Added [0.1.0] — 2025

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
