---
title: "Developer guide"
description: "Build, test, generate BPF assets, and contribute to stracectl."
weight: 13
---

This guide collects developer-focused instructions: building the binary, running
tests, generating eBPF assets, and common CI/release tasks.

## Prerequisites

- Go 1.26+ (set `GOPATH`/`GOROOT` as needed)
- `clang`, `llvm`, `libelf-dev`, matching kernel headers (for eBPF builds)
- `bpf2go` (install via `go install github.com/cilium/ebpf/cmd/bpf2go@latest`)
 - Docker (optional, to build a production image that includes both backends)

## Build locally

Build a plain binary (no eBPF):

```bash
go build -o bin/stracectl ./...
```

Install the latest command into your `$GOPATH/bin`:

```bash
go install github.com/fabianoflorentino/stracectl@latest
```

## Build with eBPF support

Generate BPF bindings and build an eBPF-enabled binary (requires `bpf2go` and kernel headers):

```bash
go install github.com/cilium/ebpf/cmd/bpf2go@latest
go generate ./internal/tracer/...
CGO_ENABLED=1 go build -tags=ebpf -o stracectl-ebpf ./...
```

Alternatively use the included Docker target to produce a production image
containing both backends:

```bash
docker build --target production -t stracectl:latest .
```

## Generate BPF (helper script)

The repository includes `scripts/generate-bpf.sh` which wraps the `bpf2go` calls
and places generated files under `internal/tracer/bpf`. Use it when regenerating
BPF helpers after editing `internal/tracer/bpf/syscall.c`.

## Tests

Run unit tests for the whole repo:

```bash
go test ./...
```

Run a single package or verbose output when debugging:

```bash
go test ./internal/tracer -v
```

## Formatting & linting

Format Go sources and run basic vet checks:

```bash
gofmt -w .
go vet ./...
```

If you use `golangci-lint`, run:

```bash
golangci-lint run
```

## CI / Release notes

- CI verifies `go test ./...`, formatting and basic linting. Keep the CI workflow green before filing a PR.
- Releases are published on GitHub Releases. Build artifacts (binaries, Docker images) are produced from the release workflow and tagged with the repository release version.

## Debugging eBPF builds

- Ensure the host kernel headers match the running kernel (`uname -r`).
- If `bpf2go` fails to compile, check `clang`/`llvm` versions and installed `libelf`.

## Working with the tracer

- `internal/tracer/strace.go` — the `strace` subprocess tracer and integration with the `parser`.
- `internal/parser` — the stateful line parser that stitches `<unfinished ...>` / `<... resumed>` lines.
- `internal/tracer/ebpf.go` — eBPF backend implementation (conditional on build tag `ebpf`).

## Contributing

Follow the [Contributing]({{< relref "docs/contributing.md" >}}) page for PR workflow, code style, and guidance on submitting patches that affect user-visible docs or behavior.
