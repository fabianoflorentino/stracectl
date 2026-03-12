#!/usr/bin/env bash
set -euo pipefail

# apply_runner_proxy.sh
# Copy a systemd drop-in to a self-hosted runner host and restart the runner service.
# Usage examples:
#   chmod +x deploy/scripts/apply_runner_proxy.sh
#   ./deploy/scripts/apply_runner_proxy.sh --host ubuntu@runner-host --service actions.runner.myorg.myrepo.myrunner.service
#   ./deploy/scripts/apply_runner_proxy.sh --host ubuntu@runner-host --service actions.runner.myorg.myrepo.myrunner.service --ssh-key ~/.ssh/id_rsa --follow

DROPIN="deploy/systemd/actions-runner-proxy.conf"
HOST=""
SERVICE=""
SSH_OPTS=""
SSH_KEY=""
FOLLOW=0
RERUN=""
GITHUB_REPO="fabianoflorentino/stracectl"

usage() {
  cat <<EOF
Usage: $0 --host user@host --service SERVICE [options]

Options:
  --host, -h      SSH target (user@host)
  --service, -s   systemd service name (e.g. actions.runner.<org>.<repo>.<runner>.service)
  --dropin, -d    local drop-in path (default: deploy/systemd/actions-runner-proxy.conf)
  --ssh-key       path to ssh private key (added to ssh/scp with -i)
  --ssh-opts      extra ssh options (quoted) passed to ssh/scp
  --follow        tail logs after restart
  --rerun RUN_ID  run 'gh run rerun RUN_ID --repo $GITHUB_REPO' locally after restart
  --help          show this help

Example:
  $0 --host ubuntu@runner-host --service actions.runner.myorg.myrepo.myrunner.service --ssh-key ~/.ssh/id_rsa --follow
EOF
}

if [[ $# -eq 0 ]]; then
  usage
  exit 1
fi

while [[ $# -gt 0 ]]; do
  case "$1" in
    --host|-h)
      HOST="$2"; shift 2;;
    --service|-s)
      SERVICE="$2"; shift 2;;
    --dropin|-d)
      DROPIN="$2"; shift 2;;
    --ssh-key)
      SSH_KEY="$2"; shift 2;;
    --ssh-opts)
      SSH_OPTS="$2"; shift 2;;
    --follow)
      FOLLOW=1; shift;;
    --rerun)
      RERUN="$2"; shift 2;;
    --help)
      usage; exit 0;;
    *)
      echo "Unknown arg: $1" >&2; usage; exit 2;;
  esac
done

if [[ -z "$HOST" || -z "$SERVICE" ]]; then
  echo "Missing required argument: --host and --service are required." >&2
  usage
  exit 2
fi

if [[ ! -f "$DROPIN" ]]; then
  echo "Drop-in file not found: $DROPIN" >&2
  exit 2
fi

SSH_CMD_OPTS=""
if [[ -n "$SSH_KEY" ]]; then
  SSH_CMD_OPTS="-i $SSH_KEY"
fi
if [[ -n "$SSH_OPTS" ]]; then
  SSH_CMD_OPTS="$SSH_CMD_OPTS $SSH_OPTS"
fi

remote_tmp="/tmp/$(basename "$DROPIN").$$"
# If service name contains slashes or spaces it's likely wrong; trust user input
dest_dir="/etc/systemd/system/${SERVICE}.d"

echo "-> Copying $DROPIN -> $HOST:$remote_tmp"
scp $SSH_CMD_OPTS "$DROPIN" "$HOST:$remote_tmp"

echo "-> Installing drop-in and restarting service $SERVICE on $HOST"
ssh $SSH_CMD_OPTS "$HOST" "sudo mkdir -p '$dest_dir' && sudo mv '$remote_tmp' '$dest_dir/http-proxy.conf' && sudo systemctl daemon-reload && sudo systemctl restart '$SERVICE' || true"

echo "-> Service status"
ssh $SSH_CMD_OPTS "$HOST" "sudo systemctl status --no-pager -l '$SERVICE' || true"

echo "-> Recent journal lines"
ssh $SSH_CMD_OPTS "$HOST" "sudo journalctl -u '$SERVICE' -n 200 --no-pager || true"

if [[ "$FOLLOW" -eq 1 ]]; then
  echo "-> Tailing logs (ctrl-C to stop)"
  ssh $SSH_CMD_OPTS "$HOST" "sudo journalctl -u '$SERVICE' -f --no-pager"
fi

if [[ -n "$RERUN" ]]; then
  echo "-> Rerunning GH workflow run id $RERUN locally"
  gh run rerun "$RERUN" --repo "$GITHUB_REPO"
fi

echo "-> Done."
