#!/usr/bin/env bash
set -euo pipefail

TAG="${1:?Usage: generate-release-notes.sh <tag>}"

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

if ! git rev-parse "$TAG" >/dev/null 2>&1; then
  echo "Tag not found: $TAG" >&2
  exit 1
fi

PREV_TAG="$(git tag -l 'v*' --sort=-version:refname | awk -v tag="$TAG" '$0 == tag { if (getline > 0) print; exit }')"

if ! command -v git-cliff >/dev/null 2>&1; then
  echo "git-cliff is required but not installed" >&2
  exit 1
fi

if [ -n "$PREV_TAG" ]; then
  RANGE="${PREV_TAG}..${TAG}"
else
  RANGE="$TAG"
fi

component_label() {
  case "$1" in
    cometline) echo "Cometline" ;;
    cometmind) echo "CometMind" ;;
    comet-sdk) echo "Comet SDK" ;;
    *) echo "$1" ;;
  esac
}

normalize_notes() {
  awk '
    /^### / {
      section = $0
      if (!(section in seen)) {
        seen[section] = 1
        order[++count] = section
      }
      next
    }
    /^- / {
      if (section != "") {
        items[section] = items[section] $0 ORS
      }
    }
    END {
      for (i = 1; i <= count; i++) {
        section = order[i]
        if (items[section] != "") {
          if (printed) {
            print ""
          }
          print section
          printf "%s", items[section]
          printed = 1
        }
      }
    }
  '
}

sections=0

for component in cometline cometmind comet-sdk; do
  notes="$(git cliff \
    --config "$ROOT/cliff.toml" \
    --strip all \
    --include-path "$component" \
    --include-path "$component/**" \
    "$RANGE" || true)"

  notes="$(printf '%s\n' "$notes" | normalize_notes)"

  if [ -n "$notes" ]; then
    if [ "$sections" -gt 0 ]; then
      echo ""
    fi
    echo "## $(component_label "$component")"
    echo "$notes"
    sections=$((sections + 1))
  fi
done

if [ "$sections" -eq 0 ]; then
  echo "No user-facing changes in this release."
fi
