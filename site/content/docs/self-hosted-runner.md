---
title: "Self-hosted runner for eBPF"
description: "How to provision a self-hosted GitHub Actions runner suitable for eBPF integration tests."
weight: 7
---

This page explains how to provision a self-hosted runner to execute eBPF
integration tests and builds. Some CI steps that load BPF programs require
kernel features or privileges not available on GitHub-hosted runners; a
self-hosted runner gives you full control of kernel, packages, and
capabilities.

Minimum requirements

- Linux kernel >= 5.8 (BPF ringbuf support)
- Go 1.26+
- Packages: `clang`, `llvm`, `build-essential`, `libelf-dev`, `libbpf-dev`,
  and matching `linux-headers-$(uname -r)`
- `bpf2go` (install with `go install github.com/cilium/ebpf/cmd/bpf2go@latest`)
- `/sys/fs/bpf` mounted (bpffs)

See the repository `docs/SELF_HOSTED_RUNNER.md` for a more complete
walkthrough (proxy drop-in, helper scripts, local VM quickstart):

[docs/SELF_HOSTED_RUNNER.md](../../docs/SELF_HOSTED_RUNNER.md)

Runner labels and workflow

Register the runner in GitHub and include the label `ebpf` (workflow uses
`runs-on: [self-hosted, linux, ebpf]`). Example job header:

```yaml
jobs:
  integration:
    runs-on: [self-hosted, linux, ebpf]
```

Privileges

Loading BPF objects often requires elevated privileges. The simplest and
most reliable option is to run the runner as root (service mode) on a
dedicated VM. If running unprivileged, ensure the account executing the
tests has the necessary kernel capabilities (e.g. `CAP_SYS_ADMIN` or
`CAP_BPF` where supported).

Bootstrap (Debian/Ubuntu example)

```bash
sudo apt-get update
sudo apt-get install -y curl git build-essential clang llvm libelf-dev libbpf-dev pkg-config linux-headers-$(uname -r)

go install github.com/cilium/ebpf/cmd/bpf2go@latest

sudo mkdir -p /sys/fs/bpf
sudo mount -t bpf bpf /sys/fs/bpf || true
```

Proxy and helper scripts

If your environment requires an HTTP/HTTPS proxy, the repository contains an
example systemd drop-in at `deploy/systemd/actions-runner-proxy.conf` and a
helper script at `deploy/scripts/apply_runner_proxy.sh` that can copy the
drop-in and restart the runner service. See `docs/SELF_HOSTED_RUNNER_PROXY.md`
for usage notes.

Register the GitHub Actions runner

Follow GitHub's official instructions to download and register the runner
for your repository or organization. When configuring the runner, add the
labels `self-hosted,linux,ebpf` (or at least `ebpf`) so the workflow can
target it.

Security

Use an isolated VM for self-hosted runners that need elevated privileges.
Rotate registration tokens, keep the host patched, and restrict network
access where possible.
