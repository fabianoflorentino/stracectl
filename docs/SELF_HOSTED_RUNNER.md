# Self-hosted runner for eBPF

This document explains how to provision and configure a self-hosted GitHub Actions runner to perform builds and integration tests that load or interact with eBPF in the Linux kernel.

Why a self-hosted runner

- GitHub-hosted runners are convenient for standard builds and CI, but they do not allow privileged kernel operations (capabilities, privileged containers, etc.).
- Integration tests that load BPF programs require kernel capabilities such as `CAP_BPF`, `CAP_PERFMON`, or running as `root`.

Minimum host requirements

- A modern Linux kernel (recommended >= 5.8). Check with `uname -r`.
- Packages: `clang`, `llvm`, `libbpf-dev`, `linux-headers-$(uname -r)`, `make`, `git`, `go`.
- Sufficient disk space for toolchains and build artifacts.

Quick runner install

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

Systemd: exporting proxy environment variables (example)
If the runner sits behind an HTTP/HTTPS proxy, create a systemd drop-in to inject proxy environment variables:

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

Workflow labels for eBPF jobs
In workflows we use the `ebpf` label. Jobs that need a runner capable of loading or running eBPF should specify:

```yaml
runs-on: [self-hosted, linux, ebpf]
```

Security and privileges

- Loading eBPF programs requires kernel privileges. The simplest approach is to run the runner service as `root`.
Alternatives:
  - Run jobs inside a dedicated VM (recommended) or a privileged container with `--privileged` and appropriate capabilities.
  - Grant specific capabilities (for example `CAP_BPF`, `CAP_PERFMON`) to the testing process (more advanced).

Install `bpf2go` and local tooling

```bash
export PATH=$PATH:$(go env GOPATH)/bin
go install github.com/cilium/ebpf/cmd/bpf2go@latest
```

Diagnostics (if the job is "Waiting for a runner")

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

Collect logs if downloads fail with 401

1. Save recent runner logs for analysis on the runner host:

```bash
sudo journalctl -u actions.runner.* --since "10 minutes ago" > /tmp/runner_journal.txt
```

2. Reproduce the failed fetch (replace the failed URL shown in the logs) to capture HTTP headers:

```bash
curl -v -D /tmp/curl_headers.txt -o /tmp/curl_body.bin 'https://api.github.com/repos/actions/setup-go/tarball/<SHA>' -H 'Accept: application/vnd.github+json'
```

3. Attach `/tmp/runner_journal.txt` and `/tmp/curl_headers.txt` to the issue or PR for analysis.

Notes

- I recommend provisioning the runner in a dedicated VM (cloud) to simplify privileges and isolation.
- Use labels such as `ebpf` to target sensitive jobs.

If you want, I can:

- add this file to the repo and push the change, and
- trigger the `eBPF Build & Generate` workflow and monitor the `integration` job until it is picked up or fails (I can collect logs if needed).
Notes
