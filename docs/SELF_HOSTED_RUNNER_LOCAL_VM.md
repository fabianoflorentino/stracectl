# Local VM quickstart for self-hosted eBPF runner

This quickstart shows how to provision a local VM and install a GitHub Actions
self-hosted runner labeled `ebpf`. The example uses Multipass for convenience,
but any VM provider (VirtualBox, libvirt, cloud VM) works.

Prerequisites

- `gh` CLI authenticated for the repository.
- A VM provider (Multipass, VirtualBox, libvirt, cloud provider).
- Internet access from the VM to download the Actions runner release and
  packages.

Multipass example (recommended for quick local testing)

1. Install Multipass (Ubuntu example):

```bash
sudo snap install multipass --classic
```

2. Launch an Ubuntu VM for the runner:

```bash
multipass launch --name stracectl-runner --cpus 2 --mem 4G --disk 20G ubuntu:22.04
```

3. Install build prerequisites inside the VM:

```bash
multipass exec stracectl-runner -- bash -lc "sudo apt update && sudo apt install -y curl git build-essential clang llvm libbpf-dev linux-headers-$(uname -r) make golang-go"
```

4. Generate a registration token locally (with `gh`) and use it inside the VM:

```bash
TOKEN=$(gh api -X POST /repos/OWNER/REPO/actions/runners/registration-token --jq .token)

multipass exec stracectl-runner -- bash -lc '
  curl -LO https://github.com/actions/runner/releases/latest/download/actions-runner-linux-x64.tar.gz
  tar xzf actions-runner-linux-x64.tar.gz
  cd actions-runner
  ./config.sh --url https://github.com/OWNER/REPO --token "$TOKEN" --labels ebpf --name multipass-runner --unattended
  sudo ./svc.sh install
  sudo ./svc.sh start
'
```

5. Verify the runner appears in GitHub repo Settings → Actions → Runners with
   the `ebpf` label.

Manual VM (libvirt / VirtualBox / cloud)

- Create a Linux VM (Ubuntu 22.04/24.04 suggested) with at least 2 vCPUs,
  4GB RAM and 20GB disk.
- SSH into the VM, install the same prerequisites and register the runner
  using the `./config.sh` step shown above.

Notes and tips

- Use `uname -r` inside the VM to ensure `linux-headers-$(uname -r)` installs
  correctly.
- If your environment requires an HTTP proxy, apply the systemd drop-in from
  `docs/SELF_HOSTED_RUNNER_PROXY.md` (the repository also contains
  `deploy/systemd/actions-runner-proxy.conf` and
  `deploy/scripts/apply_runner_proxy.sh` to help automate this).
- Keep the runner dedicated to trusted CI workloads. Runners execute workflow
  code from your repository and can run arbitrary commands.

Next steps

- After the runner is online and labeled `ebpf`, restart the `eBPF Build &
  Generate` workflow (or other workflows that use `runs-on: [self-hosted, linux, ebpf]`).
- If action downloads fail with 401, collect the runner journal and curl
  headers as described in `docs/SELF_HOSTED_RUNNER.md` and attach them when
  requesting help.
