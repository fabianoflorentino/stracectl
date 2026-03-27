UI refactor — Entrypoint and cleanup

This commit documents the decision to keep the Bubble Tea `Run` entrypoint in
`internal/ui/tui.go` and remove a temporary `internal/ui/app/run.go` file used
during iterative refactoring.

- `Run` and `runWithOpts` are implemented in `internal/ui/tui.go` to avoid an
  import cycle during refactor.
- The temporary `internal/ui/app` file was removed to keep a single, small
  entrypoint for the TUI. Future extraction of app-level wiring should ensure
  no import cycles are introduced.
