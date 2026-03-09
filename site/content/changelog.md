---
title: "Changelog"
description: "All notable changes to stracectl."
---

All notable changes to stracectl are documented here.
The format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

---

## v1.0.35 — 2026-03-09

### Added

**GitHub Pages site** — A fully custom, animated Hugo site is now live at [fabianoflorentino.github.io/stracectl](https://fabianoflorentino.github.io/stracectl/), with documentation, changelog, roadmap, and a hero terminal animation. A dedicated GitHub Actions workflow (`pages.yml`) builds and deploys the site on every push to `site/**`.

**`.dockerignore`** — A `.dockerignore` file excludes `site/`, `docs/`, `deploy/`, `scripts/`, and other non-build assets from the Docker build context, reducing context size and speeding up image builds.

---

## v1.0.34 — 2026-03-09

### Added

**Docker Hub tag cleanup** — A new `cleanup.yml` workflow (and `scripts/cleanup-dockerhub-tags.sh`) removes stale Docker Hub image tags, keeping only the latest release. Supports a `dry_run` mode to preview deletions before applying them.

**Production Docker target** — The Docker build command now explicitly targets the `production` stage (`--target production`), preventing accidental builds of the development image.

### Fixed

**README badges** — Corrected and standardised shield badges for consistency across all CI/CD and package links.

**Dockerfile comments** — Clarified stage descriptions and build example comments for readability.

---

## v1.0.33 — 2026-03-09

### Added

**GitHub release cleanup** — A new `cleanup-releases.sh` script and `Cleanup` workflow prune old GitHub releases and tags, retaining only the latest. A `dry_run` input lets you preview what would be deleted before committing.

### Changed

**Linux-specific CI removed** — The separate Linux CI workflow was consolidated into the main CI pipeline.

---

## v1.0.26 – v1.0.32 — 2026-02-xx

### Added

**CI hardening** — All GitHub Actions steps are now pinned to immutable commit SHAs to prevent supply-chain attacks. A `dependency-review` job blocks PRs that introduce dependencies with known moderate-or-higher vulnerabilities. A Semgrep SAST job runs the Go ruleset on every push.

**CODEOWNERS** — A `CODEOWNERS` file enforces mandatory code review for all paths.

**Dependabot** — Automated dependency update PRs configured for Go modules and GitHub Actions.

**`go mod tidy` pre-push hook** — `lefthook` now runs `go mod tidy` before every push to keep the module graph clean.

### Fixed

**`paths-ignore` for `.github/`** — CI `push`/`pull_request` triggers now correctly skip workflow runs when only `.github/` files change.

**Help overlay alignment** — The `COMMON PATTERNS` section in the TUI help overlay is now correctly indented.

---

## v1.0.24 – v1.0.25 — 2026-02-xx

### Added

**`stracectl stats` — post-mortem analysis** — A new `stats` sub-command reads a saved strace output file and produces the same aggregated syscall statistics (counts, latencies, error rates, percentiles) that the live TUI shows, without needing a running process.

**`--report` flag** — `stracectl stats --report` generates a self-contained, single-file HTML report with the full syscall breakdown. The report embeds all CSS and JS inline so it can be shared without extra assets.

### Fixed

**Scanner buffer too small for large strace lines** — The default `bufio.Scanner` token buffer (64 KiB) was exceeded by `strace` lines containing large read/write dumps. Fixed by extracting file-loading into `loadAggFromFile` and increasing the buffer to 512 KiB — matching the limit used by the live tracer.

**TUI frozen after traced process exits** — Two root causes fixed: (1) `exec.CommandContext` only killed the `strace` process, leaving long-running child processes alive and the stderr pipe open; fixed by setting `Setpgid: true` and sending `SIGKILL` to the entire process group on cancel. (2) Sending `tea.QuitMsg` on process exit caused the TUI to vanish immediately; replaced with `processDeadMsg` so the footer shows an amber *"✔ process exited — press q to quit"* banner instead.

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
