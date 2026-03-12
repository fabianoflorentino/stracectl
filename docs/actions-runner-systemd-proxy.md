# Systemd drop-in to set HTTP(S)_PROXY for a GitHub Actions runner

This file provides an example `systemd` drop-in that exports `HTTP_PROXY`,
`HTTPS_PROXY`, and `NO_PROXY` for a GitHub Actions self-hosted runner service.

Important: replace `<RUNNER_SERVICE>` with your runner service name. Commonly
the runner service name looks like `actions.runner.<owner>-<repo>.<runner>`.
Use `systemctl list-units | grep actions.runner` to find the exact name.

-- Example drop-in file --

Place the file at:

```
/etc/systemd/system/<RUNNER_SERVICE>.service.d/http-proxy.conf
```

Contents (example):

```ini
[Service]
Environment="HTTP_PROXY=http://proxy.example.com:3128"
Environment="HTTPS_PROXY=http://proxy.example.com:3128"
Environment="NO_PROXY=localhost,127.0.0.1,github.com,api.github.com,codeload.github.com,raw.githubusercontent.com"
```

If your proxy requires credentials, include them in the URL (URL-encode if
necessary):

```
HTTP_PROXY=http://username:password@proxy.example.com:3128
```

Commands to install and restart the runner service with the drop-in:

```bash
# create drop-in directory (replace <RUNNER_SERVICE>)
sudo mkdir -p /etc/systemd/system/<RUNNER_SERVICE>.service.d

# write the drop-in file (replace the values above and RUNNER_SERVICE)
sudo tee /etc/systemd/system/<RUNNER_SERVICE>.service.d/http-proxy.conf > /dev/null <<'EOF'
[Service]
Environment="HTTP_PROXY=http://proxy.example.com:3128"
Environment="HTTPS_PROXY=http://proxy.example.com:3128"
Environment="NO_PROXY=localhost,127.0.0.1,github.com,api.github.com,codeload.github.com,raw.githubusercontent.com"
EOF

# reload systemd and restart the runner service
sudo systemctl daemon-reload
sudo systemctl restart <RUNNER_SERVICE>.service

# verify environment applied
systemctl show -p Environment <RUNNER_SERVICE>.service
```

Notes
- Make sure `NO_PROXY` includes `api.github.com`, `github.com`, `codeload.github.com`, and `raw.githubusercontent.com` so action downloads bypass the proxy when appropriate.
- If your environment uses an authenticating proxy, prefer creating a runner VM behind a transparent proxy or use a runner image that has the proxy credentials securely provisioned.
- If you run the runner in a container, pass the env variables to the container runtime instead of a systemd drop-in.

If you want, I can also generate a `systemd` drop-in example for your specific runner name (paste `systemctl list-units | grep actions.runner` output) or commit this file into a different path.
