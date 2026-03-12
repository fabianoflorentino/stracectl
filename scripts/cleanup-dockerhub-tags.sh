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

# ── list tags ────────────────────────────────────────────────────────────────

log "Listing tags..."

tags_json=$(curl -fsSL \
  -H "Authorization: Bearer ${jwt}" \
  "https://hub.docker.com/v2/repositories/${NAMESPACE}/${REPOSITORY}/tags/?page_size=100")

total=$(echo "$tags_json" | jq '.count')

log "Total tags found: $total"

if [[ "$total" -eq 0 ]]; then
  log "No tags found. Nothing to do."
  exit 0
fi


# ── resolve tag names sorted by last_updated (most recent first) ────────────
# Strategy:
# - Sort tags by last_updated (most recent first)
# - Keep the `latest` alias if present
# - For tags matching semver (v?MAJOR.MINOR[.PATCH...]) keep the most recent
#   tag for each MAJOR.MINOR group
# - For non-semver tags, keep only the single most recent one

all_tags_desc=$(echo "$tags_json" | jq -r '[.results | sort_by(.last_updated) | reverse | .[].name] | .[]')
tag_count=$(echo "$tags_json" | jq '.count')

if [[ "$tag_count" -le 1 ]]; then
  only=$(echo "$all_tags_desc" | head -1)
  log "Only one tag found ($only). Nothing to do."
  exit 0
fi

declare -A seen_groups
to_delete=()

# Iterate tags from most recent to oldest, selecting one per group
while IFS= read -r tag; do
  [[ -z "$tag" ]] && continue
  if [[ "$tag" == "latest" ]]; then
    log "Keeping alias tag: latest"
    continue
  fi

  if [[ "$tag" =~ ^v?([0-9]+)\.([0-9]+) ]]; then
    group="${BASH_REMATCH[1]}.${BASH_REMATCH[2]}"
  else
    group="__other__"
  fi

  if [[ -z "${seen_groups[$group]:-}" ]]; then
    seen_groups[$group]=1
    log "Keeping tag: $tag (group: $group)"
  else
    to_delete+=("$tag")
  fi
done <<< "$all_tags_desc"

if [[ ${#to_delete[@]} -eq 0 ]]; then
  log "No tags to remove after grouping."
  exit 0
fi

deleted=0
failed=0

for tag in "${to_delete[@]}"; do
  if [[ "${DRY_RUN:-}" == "1" ]]; then
    log "DRY_RUN: would remove tag: $tag"
    deleted=$((deleted + 1))
    continue
  fi

  log "Removing tag: $tag"
  http_status=$(curl -o /dev/null -w "%{http_code}" -sSL \
    -X DELETE \
    -H "Authorization: Bearer ${jwt}" \
    "https://hub.docker.com/v2/repositories/${NAMESPACE}/${REPOSITORY}/tags/${tag}/")

  if [[ "$http_status" == "204" ]]; then
    deleted=$((deleted + 1))
  else
    warn "Failed to remove tag '$tag' (HTTP $http_status)"
    failed=$((failed + 1))
  fi
done

# ── summary ───────────────────────────────────────────────────────────────────

echo ""
log "Done. Removed: $deleted | Failed: $failed | Kept: $latest_tag"

if [[ "$failed" -gt 0 ]]; then
  exit 1
fi
