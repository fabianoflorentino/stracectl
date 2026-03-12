#!/usr/bin/env bash
# cleanup-releases.sh — Removes all GitHub releases and tags,
# keeping only the latest one. If there is only one, it does nothing.
#
# Dependencies: gh (GitHub CLI), jq
# Usage: ./scripts/cleanup-releases.sh [OWNER/REPO]
#   OWNER/REPO is optional; if omitted, it is inferred via `gh repo view`.

set -euo pipefail

# ── helpers ──────────────────────────────────────────────────────────────────

log()  { echo "[INFO]  $*"; }
warn() { echo "[WARN]  $*" >&2; }
die()  { echo "[ERROR] $*" >&2; exit 1; }

# ── dependencies ─────────────────────────────────────────────────────────────

command -v gh  >/dev/null 2>&1 || die "gh (GitHub CLI) not found. Install it at: https://cli.github.com"
command -v jq  >/dev/null 2>&1 || die "jq not found. Install it with your package manager."

# ── repository ───────────────────────────────────────────────────────────────

if [[ $# -gt 0 ]]; then
  REPO="$1"
else
  REPO=$(gh repo view --json nameWithOwner --jq '.nameWithOwner' 2>/dev/null) \
    || die "Could not detect the repository. Pass OWNER/REPO as an argument."
fi

log "Repository: $REPO"

# ── list releases (sorted by creation date, most recent first) ───────────────

log "Listing releases..."


releases=$(gh api "repos/${REPO}/releases?per_page=100" \
  --jq '[.[] | {tag: .tag_name, id: .id, published: .published_at, prerelease: .prerelease}]') \

total=$(echo "$releases" | jq 'length')

log "Total releases found: $total"

if [[ "$total" -eq 0 ]]; then
  log "No releases found. Nothing to do."
  exit 0
fi

if [[ "$total" -eq 1 ]]; then
  only=$(echo "$releases" | jq -r '.[0].tag')
  log "Only one release found ($only). Nothing to do."
  exit 0
fi

# ── identify the latest release (marked as Latest by GitHub) ────────────────

latest_tag=$(gh api "repos/${REPO}/releases/latest" --jq '.tag_name' 2>/dev/null) || true

if [[ -z "$latest_tag" ]]; then
  # Fallback: use the first in the list (most recent by published date)
  latest_tag=$(echo "$releases" | jq -r '.[0].tag')
  warn "No release marked as Latest. Using most recent by date: $latest_tag"
fi

log "Latest release: $latest_tag"

# ── keep one release per major.minor (semver) and one for non-semver group ──
# Strategy:
# - Sort releases by published date (most recent first)
# - For tags matching semver (v?MAJOR.MINOR[.PATCH...]) keep the most recent
#   release for each MAJOR.MINOR group
# - For non-semver tags, keep only the most recent one
# - Always keep the release pointed by GitHub as `latest` if present

declare -A seen_groups
to_delete=()

# Mark the latest tag's group as kept so it will not be deleted
if [[ -n "$latest_tag" ]]; then
  if [[ "$latest_tag" =~ ^v?([0-9]+)\.([0-9]+) ]]; then
    lg="${BASH_REMATCH[1]}.${BASH_REMATCH[2]}"
  else
    lg="__other__"
  fi
  seen_groups["$lg"]=1
  log "Keeping latest tag: $latest_tag (group: $lg)"
fi

# Iterate releases ordered by published date (most recent first)
while IFS= read -r entry; do
  tag=$(echo "$entry" | jq -r '.tag')
  if [[ "$tag" == "$latest_tag" ]]; then
    continue
  fi

  if [[ "$tag" =~ ^v?([0-9]+)\.([0-9]+) ]]; then
    group="${BASH_REMATCH[1]}.${BASH_REMATCH[2]}"
  else
    group="__other__"
  fi

  if [[ -z "${seen_groups[$group]:-}" ]]; then
    seen_groups["$group"]=1
    log "Keeping tag: $tag (group: $group)"
  else
    to_delete+=("$tag")
  fi
done < <(echo "$releases" | jq -c 'sort_by(.published) | reverse | .[]')

if [[ ${#to_delete[@]} -eq 0 ]]; then
  log "No releases to remove after grouping."
  exit 0
fi

deleted=0
failed=0

for tag in "${to_delete[@]}"; do
  if [[ "${DRY_RUN:-}" == "1" ]]; then
    log "DRY_RUN: would remove release and tag: $tag"
    deleted=$((deleted + 1))
    continue
  fi

  log "Removing release and tag: $tag"
  if gh release delete "$tag" \
       --repo "$REPO" \
       --yes \
       --cleanup-tag 2>&1; then
    deleted=$((deleted + 1))
  else
    warn "Failed to remove: $tag"
    failed=$((failed + 1))
  fi
done

# ── summary ──────────────────────────────────────────────────────────────────

echo ""
log "Done. Removed: $deleted | Failed: $failed | Kept: $latest_tag"

if [[ "$failed" -gt 0 ]]; then
  exit 1
fi
