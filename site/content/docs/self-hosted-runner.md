---
title: "Self-hosted runner"
description: "How to provision a self-hosted GitHub Actions runner and integrate `stracectl` (eBPF and strace)."
weight: 12
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
This page contains a self-contained walkthrough. For repository examples and
helper scripts see `deploy/` and the `docs/` folder.

## Quick runner install

1. Create a directory for the runner and download the official release:

```bash
mkdir -p ~/actions-runner && cd ~/actions-runner
RUNNER_VER=2.305.0
curl -sL -o actions-runner.tar.gz \
  https://github.com/actions/runner/releases/download/v${RUNNER_VER}/actions-runner-linux-x64-${RUNNER_VER}.tar.gz
tar xzf actions-runner.tar.gz
```

2. Register the runner with your repository (generate a temporary token in the GitHub UI: Settings → Actions → Runners → New self-hosted runner):

```bash
./config.sh --url https://github.com/ORG/REPO --token YOUR_TOKEN --labels self-hosted,linux,ebpf --name my-runner
```

3. Install and start the runner as a system service (run the following as `root` to install the systemd unit):

```bash
sudo ./svc.sh install
sudo ./svc.sh start
```

## Systemd: exporting proxy environment variables

If the runner sits behind an HTTP/HTTPS proxy, create a systemd drop-in to
inject proxy environment variables (example unit shown):

```bash
SERVICE=actions.runner.ORG-REPO.my-runner.service
sudo mkdir -p /etc/systemd/system/${SERVICE}.d
sudo tee /etc/systemd/system/${SERVICE}.d/proxy.conf > /dev/null <<'EOF'
[Service]
Environment="HTTP_PROXY=http://proxy.example:3128"
Environment="HTTPS_PROXY=http://proxy.example:3128"
Environment="NO_PROXY=localhost,127.0.0.1,github.com,api.github.com"
Environment="http_proxy=http://proxy.example:3128"
Environment="https_proxy=http://proxy.example:3128"
Environment="no_proxy=localhost,127.0.0.1,github.com,api.github.com"
EOF

sudo systemctl daemon-reload
sudo systemctl restart "${SERVICE}"
sudo journalctl -u "${SERVICE}" -f
```

Notes:

- Edit `deploy/systemd/actions-runner-proxy.conf` in the repo to match your proxy endpoints and `NO_PROXY` entries. Keep GitHub hostnames (github.com, api.github.com, raw.githubusercontent.com) in `NO_PROXY` if the proxy should be bypassed for those hosts.
- The repo includes `deploy/scripts/apply_runner_proxy.sh` to help copy the drop-in and restart the service on a remote runner host.

## Workflow labels for eBPF jobs

Jobs that need a runner capable of loading or running eBPF should specify the `ebpf` label:

```yaml
runs-on: [self-hosted, linux, ebpf]
```

## Security and privileges

- Loading eBPF programs requires kernel privileges. The simplest approach is to run the runner service as `root` on a dedicated VM.
- Alternatives: run jobs inside a dedicated VM (recommended) or a privileged container with `--privileged` and appropriate capabilities, or grant specific capabilities (for example `CAP_BPF`, `CAP_PERFMON`) to the testing process.

## Install `bpf2go` and local tooling

```bash
export PATH=$PATH:$(go env GOPATH)/bin
go install github.com/cilium/ebpf/cmd/bpf2go@latest
```

## Diagnostics (if the job is "Waiting for a runner")

- List repository runners (shows name, status and labels):

```bash
gh api repos/ORG/REPO/actions/runners --jq '.runners[] | {name: .name, status: .status, busy: .busy, labels: .labels}'
```

- On the runner host, check the systemd service and logs:

```bash
systemctl list-units --type=service | grep actions.runner
sudo systemctl status actions.runner.* --no-pager
sudo journalctl -u actions.runner.* --since "10 minutes ago"
```

If the runner is missing the `ebpf` label, reconfigure it with the correct labels:

```bash
# in the runner directory
./config.sh remove --unattended
./config.sh --url https://github.com/ORG/REPO --token YOUR_TOKEN --labels self-hosted,linux,ebpf --name my-runner
sudo ./svc.sh start
```

## Collect logs if downloads fail with 401

1. Save recent runner logs for analysis on the runner host:

```bash
sudo journalctl -u actions.runner.* --since "10 minutes ago" > /tmp/runner_journal.txt
```

2. Reproduce the failed fetch (replace the failed URL shown in the logs) to capture HTTP headers:

```bash
curl -v -D /tmp/curl_headers.txt -o /tmp/curl_body.bin 'https://api.github.com/repos/actions/setup-go/tarball/<SHA>' -H 'Accept: application/vnd.github+json'
```

3. Attach `/tmp/runner_journal.txt` and `/tmp/curl_headers.txt` to the issue or PR for analysis.

## Local VM quickstart

This quickstart shows how to provision a local VM and install a GitHub Actions
self-hosted runner labeled `ebpf`. The example uses Multipass for convenience,
but any VM provider works.

### Multipass example

```bash
sudo snap install multipass --classic
multipass launch --name stracectl-runner --cpus 2 --mem 4G --disk 20G ubuntu:22.04

multipass exec stracectl-runner -- bash -lc "sudo apt update && sudo apt install -y curl git build-essential clang llvm libbpf-dev linux-headers-$(uname -r) make golang-go"

TOKEN=$(gh api -X POST /repos/OWNER/REPO/actions/runners/registration-token --jq .token)

multipass exec stracectl-runner -- bash -lc '
  curl -LO https://github.com/actions/runner/releases/latest/download/actions-runner-linux-x64.tar.gz
  tar xzf actions-runner-linux-x64.tar.gz
  cd actions-runner
  ./config.sh --url https://github.com/OWNER/REPO --token "$TOKEN" --labels ebpf --name multipass-runner --unattended
  sudo ./svc.sh install
  sudo ./svc.sh start
'

# Verify the runner appears in GitHub repo Settings → Actions → Runners with the `ebpf` label.
```

Notes and tips:

- Use `uname -r` inside the VM to ensure `linux-headers-$(uname -r)` installs correctly.
- If your environment requires an HTTP proxy, apply the systemd drop-in from `deploy/systemd/actions-runner-proxy.conf` (the repository also contains `deploy/scripts/apply_runner_proxy.sh`).
- Keep the runner dedicated to trusted CI workloads. Runners execute workflow code from your repository and can run arbitrary commands.

## Notes

- I recommend provisioning the runner in a dedicated VM (cloud) to simplify privileges and isolation.
- Use labels such as `ebpf` to target sensitive jobs.

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
