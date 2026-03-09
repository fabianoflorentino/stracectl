#!/usr/bin/env bash
# cleanup-dockerhub-tags.sh — Removes all Docker Hub image tags,
# keeping only the most recently pushed one. If there is only one, it does nothing.
#
# Dependencies: curl, jq
# Usage: ./scripts/cleanup-dockerhub-tags.sh [NAMESPACE/REPOSITORY]
#   NAMESPACE/REPOSITORY is optional; if omitted, it is inferred from the git remote URL.
#
# Required environment variables:
#   DOCKERHUB_USERNAME  — Docker Hub login name
#   DOCKERHUB_TOKEN     — Docker Hub access token or password

set -euo pipefail

# ── helpers ───────────────────────────────────────────────────────────────────

log()  { echo "[INFO]  $*"; }
warn() { echo "[WARN]  $*" >&2; }
die()  { echo "[ERROR] $*" >&2; exit 1; }

# ── dependencies ──────────────────────────────────────────────────────────────

command -v curl >/dev/null 2>&1 || die "curl not found. Install it with your package manager."
command -v jq   >/dev/null 2>&1 || die "jq not found. Install it with your package manager."

# ── credentials ───────────────────────────────────────────────────────────────

[[ -z "${DOCKERHUB_USERNAME:-}" ]] && die "DOCKERHUB_USERNAME is not set."
[[ -z "${DOCKERHUB_TOKEN:-}"    ]] && die "DOCKERHUB_TOKEN is not set."

# ── repository ────────────────────────────────────────────────────────────────

if [[ $# -gt 0 ]]; then
  IMAGE="$1"
else
  remote_url=$(git remote get-url origin 2>/dev/null) \
    || die "Could not detect git remote. Pass NAMESPACE/REPOSITORY as an argument."
  # Extract "owner/repo" from https://github.com/owner/repo.git or git@github.com:owner/repo.git
  IMAGE=$(echo "$remote_url" | sed -E 's|.*[:/]([^/]+/[^/]+)(\.git)?$|\1|')
  [[ -z "$IMAGE" ]] && die "Could not parse repository from remote URL: $remote_url"
fi

NAMESPACE="${IMAGE%%/*}"
REPOSITORY="${IMAGE##*/}"

log "Docker Hub image: $NAMESPACE/$REPOSITORY"

# ── authenticate ──────────────────────────────────────────────────────────────

log "Authenticating with Docker Hub..."

jwt=$(curl -fsSL \
  -X POST \
  -H "Content-Type: application/json" \
  -d "{\"username\": \"${DOCKERHUB_USERNAME}\", \"password\": \"${DOCKERHUB_TOKEN}\"}" \
  "https://hub.docker.com/v2/users/login" \
  | jq -r '.token') \
  || die "Authentication failed. Check your DOCKERHUB_USERNAME and DOCKERHUB_TOKEN."

[[ -z "$jwt" || "$jwt" == "null" ]] && die "Received empty token from Docker Hub."

# ── list tags (sorted by last_updated descending — API default) ───────────────

log "Listing tags..."

tags_json=$(curl -fsSL \
  -H "Authorization: Bearer ${jwt}" \
  "https://hub.docker.com/v2/repositories/${NAMESPACE}/${REPOSITORY}/tags/?page_size=100&ordering=-last_updated")

total=$(echo "$tags_json" | jq '.count')

log "Total tags found: $total"

if [[ "$total" -eq 0 ]]; then
  log "No tags found. Nothing to do."
  exit 0
fi

# ── resolve tag names from the first page ────────────────────────────────────

all_tags=$(echo "$tags_json" | jq -r '[.results[] | .name] | .[]')
tag_count=$(echo "$all_tags" | wc -l | tr -d ' ')

if [[ "$tag_count" -le 1 ]]; then
  only=$(echo "$all_tags" | head -1)
  log "Only one tag found ($only). Nothing to do."
  exit 0
fi

# ── identify the most recently pushed tag (first in the ordered list) ─────────
# Skip the "latest" alias if it is the first result and there are other tags.

latest_tag=$(echo "$all_tags" | grep -v '^latest$' | head -1)

if [[ -z "$latest_tag" ]]; then
  latest_tag=$(echo "$all_tags" | head -1)
  warn "Only a 'latest' alias tag found. Keeping it and skipping."
  exit 0
fi

log "Tag kept (most recent): $latest_tag"

# ── remove all other tags ─────────────────────────────────────────────────────

deleted=0
failed=0

while IFS= read -r tag; do
  [[ "$tag" == "$latest_tag" ]] && continue

  log "Removing tag: $tag"
  http_status=$(curl -o /dev/null -w "%{http_code}" -fsSL \
    -X DELETE \
    -H "Authorization: Bearer ${jwt}" \
    "https://hub.docker.com/v2/repositories/${NAMESPACE}/${REPOSITORY}/tags/${tag}/")

  if [[ "$http_status" == "204" ]]; then
    deleted=$((deleted + 1))
  else
    warn "Failed to remove tag '$tag' (HTTP $http_status)"
    failed=$((failed + 1))
  fi
done <<< "$all_tags"

# ── summary ───────────────────────────────────────────────────────────────────

echo ""
log "Done. Removed: $deleted | Failed: $failed | Kept: $latest_tag"

if [[ "$failed" -gt 0 ]]; then
  exit 1
fi
