#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<EOF
Usage: $0 --version vX.Y.Z [--notes "..."] [--notes-file path] [--repo owner/repo] [--no-site]

Writes .changes/<VERSION>_release_notes.md and updates:
 - docs/CHANGELOG.md (adds link to .changes)
 - site/content/docs/changelog.md (replaces first v* link)

Script does not commit changes.
EOF
  exit 1
}

TAG=""
NOTES=""
NOTES_FILE=""
REPO=""
NO_SITE=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    -v|--version)
      TAG="$2"; shift 2 ;;
    -n|--notes)
      NOTES="$2"; shift 2 ;;
    -f|--notes-file)
      NOTES_FILE="$2"; shift 2 ;;
    -r|--repo)
      REPO="$2"; shift 2 ;;
    --no-site)
      NO_SITE=1; shift ;;
    -h|--help)
      usage ;;
    *)
      echo "Unknown option: $1" >&2; usage ;;
  esac
done

if [[ -z "${TAG}" ]]; then
  echo "Missing --version" >&2; usage
fi

if [[ -n "${NOTES_FILE}" ]]; then
  if [[ -f "${NOTES_FILE}" ]]; then
    NOTES=$(cat "${NOTES_FILE}")
  else
    echo "Notes file not found: ${NOTES_FILE}" >&2
    exit 2
  fi
fi

if [[ -z "${NOTES}" ]]; then
  NOTES="Release ${TAG}

- Release notes not provided."
fi

mkdir -p .changes
printf '%s\n' "${NOTES}" > ".changes/${TAG}_release_notes.md"
echo "Wrote .changes/${TAG}_release_notes.md"

# Update docs/CHANGELOG.md
if [[ -f docs/CHANGELOG.md ]]; then
  LINK="[${TAG}](.changes/${TAG}_release_notes.md)"
  if grep -Fq "${LINK}" docs/CHANGELOG.md; then
    echo "docs/CHANGELOG.md already contains link for ${TAG}, skipping"
  else
    awk -v link="${LINK}" 'BEGIN{added=0} {print; if(!added && $0 ~ /^## Releases/){print ""; print link; added=1}} END{if(!added){print ""; print "## Releases"; print ""; print link}}' docs/CHANGELOG.md > docs/CHANGELOG.md.tmp && mv docs/CHANGELOG.md.tmp docs/CHANGELOG.md
    echo "Updated docs/CHANGELOG.md"
  fi
else
  echo "docs/CHANGELOG.md not found, skipping"
fi

# Determine repo if not provided
if [[ -z "${REPO}" ]]; then
  remote=$(git config --get remote.origin.url 2>/dev/null || true)
  if [[ ${remote} == git@github.com:* ]]; then
    REPO=${remote#git@github.com:}
  elif [[ ${remote} == https://github.com/* ]]; then
    REPO=${remote#https://github.com/}
  elif [[ ${remote} == ssh://git@github.com/* ]]; then
    REPO=${remote#ssh://git@github.com/}
  else
    REPO=""
  fi
  REPO=${REPO%.git}
fi

if [[ ${NO_SITE} -eq 0 ]]; then
  if [[ -f site/content/docs/changelog.md ]]; then
    if [[ -n "${REPO}" ]]; then
      GHURL="https://github.com/${REPO}/blob/main/.changes/${TAG}_release_notes.md"
    else
      GHURL="https://github.com/${TAG}"
    fi
    awk -v tag="${TAG}" -v ghurl="${GHURL}" 'BEGIN{replaced=0} { if(!replaced && $0 ~ /^\[v[^\]]+\]\(/){ print "* [" tag "](" ghurl ")"; replaced=1 } else print } END{ if(!replaced){ print ""; print "* [" tag "](" ghurl ")" } }' site/content/docs/changelog.md > site/content/docs/changelog.md.tmp && mv site/content/docs/changelog.md.tmp site/content/docs/changelog.md
    echo "Updated site/content/docs/changelog.md"
  else
    echo "site/content/docs/changelog.md not found, skipping"
  fi
fi

echo "Done"
