# Runner systemd proxy drop-in

This short guide explains how to make a systemd-hosted Actions runner inherit
HTTP/HTTPS proxy environment variables using a systemd drop-in. The repository
includes an example drop-in at `deploy/systemd/actions-runner-proxy.conf`.

1) Identify the runner systemd unit name (typical pattern:
   `actions.runner.<org>.<repo>.<runner>.service`):

```bash
systemctl list-units --type=service | grep actions.runner
```

2) Copy the example drop-in from the repo into the unit's drop-in folder and
   restart the runner service (replace `<service>` with the real unit name):

```bash
sudo mkdir -p /etc/systemd/system/<service>.d
sudo cp deploy/systemd/actions-runner-proxy.conf /etc/systemd/system/<service>.d/http-proxy.conf
sudo systemctl daemon-reload
sudo systemctl restart <service>
```

3) Verify the runner logs while a workflow runs:

```bash
sudo journalctl -u <service> -f
```

Notes

- Edit `deploy/systemd/actions-runner-proxy.conf` to match your proxy endpoints
  (`HTTP_PROXY` / `HTTPS_PROXY`) and `NO_PROXY` entries. Keep GitHub hostnames
  (github.com, api.github.com, raw.githubusercontent.com) in `NO_PROXY` if the
  proxy should be bypassed for those hosts.
- If your environment uses environment files or a different init system,
  adapt the approach accordingly.
- The repository provides `deploy/scripts/apply_runner_proxy.sh` as a small SSH
  helper to copy the drop-in and restart the service on a remote runner host.
