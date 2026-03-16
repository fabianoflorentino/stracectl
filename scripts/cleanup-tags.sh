#!/usr/bin/env bash
# cleanup-tags.sh — Removes all Git tags in the GitHub repo, keeping only the most
# recently created tag. Designed to be used from CI or locally.
#
# Dependencies: gh (GitHub CLI), jq
# Usage: ./scripts/cleanup-tags.sh [OWNER/REPO]
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

# ── list tag refs ─────────────────────────────────────────────────────────────

refs_json=$(gh api "repos/${REPO}/git/refs/tags?per_page=100") || die "Failed to list tag refs"

total=$(echo "$refs_json" | jq 'length')

log "Total tag refs found: $total"

if [[ "$total" -eq 0 ]]; then
  log "No tags found. Nothing to do."
  exit 0
fi

# ── build tag -> date mapping ─────────────────────────────────────────────────

declare -a entries=()

while IFS= read -r item; do
  name=$(echo "$item" | jq -r '.ref' | sed 's|refs/tags/||')
  sha=$(echo "$item" | jq -r '.object.sha')
  type=$(echo "$item" | jq -r '.object.type')

  date=""
  if [[ "$type" == "tag" ]]; then
    # annotated tag: has tagger.date
    tagobj=$(gh api "repos/${REPO}/git/tags/${sha}" 2>/dev/null || true)
    date=$(echo "$tagobj" | jq -r '.tagger.date // empty')
  fi

  if [[ -z "$date" ]]; then
    # fallback to commit date (lightweight tag)
    commitobj=$(gh api "repos/${REPO}/commits/${sha}" 2>/dev/null || true)
    date=$(echo "$commitobj" | jq -r '.commit.committer.date // .commit.author.date // empty')
  fi

  if [[ -z "$date" || "$date" == "null" ]]; then
    warn "Could not determine date for tag $name; skipping"
    continue
  fi

  # Use ISO date as sortable prefix
  entries+=("${date}|${name}")
done < <(echo "$refs_json" | jq -c '.[]')

if [[ ${#entries[@]} -eq 0 ]]; then
  log "No tag entries with resolvable dates. Nothing to do."
  exit 0
fi

# Sort entries by date (newest first)
mapfile -t sorted < <(printf "%s
" "${entries[@]}" | sort -r)

latest_tag=$(echo "${sorted[0]}" | cut -d'|' -f2-)
log "Latest tag (kept): $latest_tag"

# Collect tags to delete (all except the latest)
to_delete=()
for i in "${!sorted[@]}"; do
  if [[ "$i" -eq 0 ]]; then
    continue
  fi
  t=$(echo "${sorted[$i]}" | cut -d'|' -f2-)
  to_delete+=("$t")
done

if [[ ${#to_delete[@]} -eq 0 ]]; then
  log "No tags to remove."
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
  if gh api --method DELETE "repos/${REPO}/git/refs/tags/${tag}" >/dev/null 2>&1; then
    deleted=$((deleted + 1))
  else
    warn "Failed to remove tag: $tag"
    failed=$((failed + 1))
  fi
done

echo ""
log "Done. Removed: $deleted | Failed: $failed | Kept: $latest_tag"

if [[ "$failed" -gt 0 ]]; then
  exit 1
fi
