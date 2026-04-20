# stracectl

A modern strace replacement with a real-time interactive TUI, per-syscall
latency statistics, percentile histograms, anomaly alerts and an HTTP API for
serving Prometheus metrics and a web dashboard.

## Highlights

- Real-time TUI and offline analysis of saved traces
- Per-syscall P95/P99, errno breakdowns, recent error samples and alerts
- Multiple backends: classic `strace` subprocess tracer or Linux eBPF (recommended)
- Optional HTTP dashboard and Prometheus metrics (`--serve`)
- Privacy-focused features for redaction and export of sanitized event logs

## Quick links

- Source: [main.go](main.go)
- CLI entrypoint and flags: [cmd/root.go](cmd/root.go)
- Build & helper targets: [Makefile](Makefile)
- User docs: [docs/USAGE.md](docs/USAGE.md)

## Requirements

- Go (project uses `go 1.26.1`) — see [go.mod](go.mod)
- Optional: clang + bpf2go to generate eBPF artifacts when using the eBPF backend
- Docker (for image targets) and Hugo (to build the docs site)

## Install / Build

Build the binary (outputs `bin/stracectl`):

```sh
make build
```

Build with eBPF support (requires clang + bpf2go):

```sh
make build-ebpf
```

Run directly with Go:

```sh
make run ARGS="run curl https://example.com"
```

Or build the Docker image:

```sh
make docker-build
```

## Usage examples

Trace a command from start (TUI):

```sh
stracectl run curl https://example.com
```

Trace and write a self-contained HTML report:

```sh
stracectl run --report out.html curl https://example.com
```

Trace and group rows by PID (useful for multi-process workloads):

```sh
stracectl run --per-pid -- python3 app.py
```

Attach to a running PID:

```sh
stracectl attach 1234
```

Attach and expose an HTTP dashboard + Prometheus metrics:

```sh
stracectl attach --serve :8080 1234
```

Analyse a saved strace file:

```sh
stracectl stats trace.log
stracectl stats --report report.html trace.log
stracectl stats --per-pid trace.log
```

Auto-discover a container PID in a Pod or provide a container name:

```sh
stracectl discover myapp
```

For more examples and options, see the CLI help or the usage docs in [docs/USAGE.md](docs/USAGE.md).

## Privacy & Redaction

`stracectl` includes several privacy-related flags to control capture and
redaction of syscall arguments and payloads (see `--no-args`, `--max-arg-size`,
`--redact-patterns`, `--privacy-level`, `--privacy-log`, etc.). Use `--full`
only with care as it may expose sensitive data. The CLI can emit newline-
delimited redacted JSON events for downstream processing.

## Development

- Run unit tests:

```sh
make test
```

- Generate BPF artifacts (requires `clang` and `bpf2go`):

```sh
make generate-bpf
```

- Format, vet and lint using the provided Make targets:

```sh
make fmt
make vet
make lint
```

- Build and serve the site locally (Hugo):

```sh
make site-dev
```

## Contributing

Contributions are welcome. Please follow the established project style,
run tests, and open a PR describing the change. See `docs/PROJECT_STRUCTURE.md`
and other files under `docs/` for project guidance.

## License

This project is licensed under the terms found in the `LICENSE` file.

## Where to read more

- Architecture and design notes: `docs/ARCHITECTURE_DIAGRAM.md`, `docs/SYSTEM_DESIGN.md`
- Usage guides and flows: `docs/USAGE.md`, `docs/LOCAL_USAGE.md`
- eBPF details: `docs/EBPF.md`
