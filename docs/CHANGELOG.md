# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [v1.0.16] - 2026-03-07

### Added

- **TUI detail overlay**: Press `d` on any syscall row to open a detail panel showing the syscall reference description and live per-syscall stats (`feat(tui)`).
- **HTML report export**: Trace results can now be exported as a self-contained HTML report (`feat`).
- **`MarshalJSON`/`UnmarshalJSON` for `Category`**: Ensures consistent JSON serialization/deserialization of category values across all output formats (`feat`).
- **`Tracer` interface**: Introduced a clean `Tracer` interface to improve testability and decouple tracer implementation from consumers (`fix`).
- **Lefthook git hooks**: Added `lefthook.yml` with automated pre-commit (vet, lint, test) and pre-push (build, test coverage, vulnerability scan) hooks (`chore`).
- **`govulncheck` in CI hooks**: Vulnerability scanning via `govulncheck ./...` is now enforced on every push (`chore(lefthook)`).

### Fixed

- **Terminal hang on exit (`attach`/`run`)**: The strace subprocess context is now cancelled when the TUI or HTTP server exits, preventing the terminal from being held after quitting stracectl (`fix(cmd)`).
- **Strace exit code surfaced**: Parse errors are now logged and the strace process exit code is properly propagated instead of being silently swallowed (`fix`).
- **HTTP server timeouts**: Added `ReadTimeout` and `WriteTimeout` to the HTTP server to prevent slow-client resource exhaustion (`fix`).
- **Docker production image**: Switched base image from `scratch` to `distroless/base` to support glibc-linked strace binaries (`fix`).
- **`noctx` linter violation**: Replaced `exec.Command` with `exec.CommandContext` in tracer tests to satisfy the `noctx` linter rule (`fix(tracer)`).
- **README badges**: Added missing Docker Hub, Linux, Trivy scan, and Release status badges (`fix`).

### Changed

- **Refactored `runTrace` helper**: Extracted a dedicated `runTrace` function, added PID > 0 validation, and ensured goroutine drain is awaited before exit (`refactor`).
- **Refactored `scanCgroup` helper**: Extracted shared scan logic to eliminate duplication between `ScanProc` and `ScanProcLowest` (`refactor`).
- **UI style formatting**: Style definitions in `tui.go` were reformatted for improved readability (`style(ui)`).

### Performance

- **Regex compilation moved to package level**: `resumedRe` is now compiled once at init time instead of on every hot-path invocation, reducing allocations during high-frequency event parsing (`perf`).

### Tests

- **Tracer unit tests**: Added pipeline tests for `StraceTracer`, covering start/stop and event forwarding (`test(tracer)`).
- **TUI unit tests**: Added comprehensive unit tests for the BubbleTea TUI package (`test(ui)`).
- **Coverage raised to ≥ 80%**: All non-cmd packages now meet the 80% coverage threshold — `aggregator` 80%, `discover` 97.2%, `models` 100%, `parser` 88.6%, `server` 89.6%, `tracer` 88.6%, `ui` 94.1% (`test`).

### Documentation

- **README updated**: Documented the new detail overlay feature, cursor navigation keybindings, and HTML export usage (`docs`).

---

## [v1.0.6] - prior release

See [GitHub Releases](https://github.com/fabianoflorentino/stracectl/releases/tag/v1.0.6) for earlier history.
