---
title: "Quickstart"
description: "Get started with stracectl in under two minutes."
weight: 2
---

This quickstart gets you a running `stracectl` session in under two minutes.

## Try the bundled binary

From the repository root (for a fast local test):

```bash
# run the included test binary
./bin/stracectl run curl https://example.com
```

## Install and run

Install via `go install` and run a tracing session:

```bash
go install github.com/fabianoflorentino/stracectl@latest
sudo stracectl run --report report.html curl https://example.com
```

## HTTP sidecar (live web dashboard)

Start the HTTP API instead of the TUI and open the dashboard in your browser:

```bash
sudo stracectl run --serve :8080 curl https://example.com
# then open http://localhost:8080
```

## Notes

- To force a specific backend: `--backend ebpf` or `--backend strace`.
- Enable verbose tracer diagnostics with `--debug` if you encounter parsing issues.

See also: [Installation]({{< relref "docs/install.md" >}}) and [Usage]({{< relref "docs/usage.md" >}}).
