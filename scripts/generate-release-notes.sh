#!/usr/bin/env bash
set -euo pipefail

TAG="${1:?Usage: generate-release-notes.sh <tag>}"

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

if ! git rev-parse "$TAG" >/dev/null 2>&1; then
  echo "Tag not found: $TAG" >&2
  exit 1
fi

if ! command -v git-cliff >/dev/null 2>&1; then
  echo "git-cliff is required but not installed" >&2
  exit 1
fi

PREV_TAG="$(git tag -l 'v*' --sort=-version:refname | awk -v tag="$TAG" '$0 == tag { if (getline > 0) print; exit }')"

submodule_label() {
  case "$1" in
    cometline) echo "Cometline" ;;
    cometmind) echo "CometMind" ;;
    comet-sdk) echo "Comet SDK" ;;
    *) echo "$1" ;;
  esac
}

sections=0

for submodule in cometline cometmind comet-sdk; do
  if [ -n "$PREV_TAG" ]; then
    prev_sha="$(git ls-tree "$PREV_TAG" "$submodule" | awk '{print $3}')"
  else
    prev_sha="$(git -C "$submodule" rev-list --max-parents=0 HEAD | tail -1)"
  fi
  curr_sha="$(git ls-tree "$TAG" "$submodule" | awk '{print $3}')"

  if [ -z "$prev_sha" ] || [ -z "$curr_sha" ] || [ "$prev_sha" = "$curr_sha" ]; then
    continue
  fi

  notes="$(git -C "$submodule" cliff --config "$ROOT/cliff.toml" --strip all "${prev_sha}..${curr_sha}" || true)"
  notes="$(printf '%s\n' "$notes" | sed '/^[[:space:]]*$/d')"

  if [ -n "$notes" ]; then
    if [ "$sections" -gt 0 ]; then
      echo ""
    fi
    echo "## $(submodule_label "$submodule")"
    echo "$notes"
    sections=$((sections + 1))
  fi
done

if [ "$sections" -eq 0 ]; then
  echo "No user-facing changes in this release."
fi
