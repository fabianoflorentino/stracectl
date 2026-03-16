---
title: "Contributing"
description: "How to contribute to stracectl: build, test, PRs, and BPF generation."
weight: 12
---

Thank you for contributing to stracectl! Contributions are welcome — whether
it's a bugfix, documentation improvement, test, or new feature.

Getting started
---------------

- Fork the repository and create a descriptive branch: `git checkout -b fix/issue-123`.
- Run tests locally: `go test ./...`.
- Format Go code: `gofmt -w .` and prefer `go vet` for quick checks.

Build and run
-------------

Build the binary locally:

```bash
go build -o stracectl ./...
```

Install for quick testing:

```bash
go install github.com/fabianoflorentino/stracectl@latest
```

BPF / eBPF development
----------------------

To work with the eBPF tracer you may need additional tooling:

- Install `bpf2go` and build prerequisites (clang, llvm, kernel headers):

```bash
go install github.com/cilium/ebpf/cmd/bpf2go@latest
go generate ./internal/tracer/...
CGO_ENABLED=1 go build -tags=ebpf ./...
```

- The repository includes helper scripts (`scripts/generate-bpf.sh`) and a
  Docker target (`production-ebpf`) in the `Dockerfile` to produce an
  eBPF-enabled binary.

Submitting a pull request
-------------------------

- Open a PR from your fork and include a clear description and test steps.
- If the change touches user-visible behavior, add or update documentation
  under `site/content/docs/`.
- Link relevant issues and include test cases when applicable.

Reporting issues
----------------

- For bugs and feature requests, open a GitHub issue in this repository.
- For security-sensitive issues, follow the instructions in the Security
  page (do not open a public issue). See [Security]({{< relref "docs/security.md" >}}).

Thank you — your contributions make this project better for everyone.
