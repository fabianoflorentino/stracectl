UI refactor — Package extraction

This commit groups the refactor that extracted rendering, overlays, widgets,
helpers, styles, terminal utilities, and input handling from `internal/ui/tui.go`
into focused packages under `internal/ui/`.

Files moved/created include:
- internal/ui/render/*
- internal/ui/overlays/*
- internal/ui/widgets/*
- internal/ui/helpers/*
- internal/ui/styles/*
- internal/ui/terminal/*
- internal/ui/input/*

Rationale:
- Improve single-responsibility and testability.
- Make rendering and overlay logic reusable and easier to maintain.

Notes:
- See subsequent docs for controller/model abstractions and entrypoint handling.
