#!/usr/bin/env bash
set -euo pipefail

TAG="${1:?Usage: publish-homebrew-tap.sh <tag> <sha256> <asset_name> <tap_repo> [download_repo]}"
SHA="${2:?Usage: publish-homebrew-tap.sh <tag> <sha256> <asset_name> <tap_repo> [download_repo]}"
ASSET_NAME="${3:?Usage: publish-homebrew-tap.sh <tag> <sha256> <asset_name> <tap_repo> [download_repo]}"
TAP_REPO="${4:?Usage: publish-homebrew-tap.sh <tag> <sha256> <asset_name> <tap_repo> [download_repo]}"
DOWNLOAD_REPO="${5:-cometline/cometline}"

: "${GH_TOKEN:?GH_TOKEN is required to push to the tap repository}"

tap_name() {
  local owner repo short
  owner="${TAP_REPO%%/*}"
  repo="${TAP_REPO#*/}"
  short="${repo#homebrew-}"
  printf '%s/%s' "$owner" "$short"
}

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
WORKDIR="$(mktemp -d)"
trap 'rm -rf "$WORKDIR"' EXIT

gh repo clone "$TAP_REPO" "$WORKDIR/tap"
git -C "$WORKDIR/tap" remote set-url origin "https://x-access-token:${GH_TOKEN}@github.com/${TAP_REPO}.git"

mkdir -p "$WORKDIR/tap/Casks"
"$ROOT/scripts/render-homebrew-cask.sh" "$TAG" "$SHA" "$ASSET_NAME" "$DOWNLOAD_REPO" > "$WORKDIR/tap/Casks/cometline.rb"

if [ ! -f "$WORKDIR/tap/README.md" ]; then
  TAP_NAME="$(tap_name)"
  cat > "$WORKDIR/tap/README.md" <<EOF
# Homebrew tap for Cometline

Install with:

\`brew tap ${TAP_NAME}\`
\`brew install --cask cometline\`

Or in one command:

\`brew install --cask ${TAP_NAME}/cometline\`
EOF
fi

git -C "$WORKDIR/tap" config user.name "github-actions[bot]"
git -C "$WORKDIR/tap" config user.email "41898282+github-actions[bot]@users.noreply.github.com"

if [ -z "$(git -C "$WORKDIR/tap" status --short -- README.md Casks/cometline.rb)" ]; then
  echo "Homebrew tap already up to date"
  exit 0
fi

git -C "$WORKDIR/tap" add README.md Casks/cometline.rb
git -C "$WORKDIR/tap" commit -m "cometline: update to ${TAG#v}"
git -C "$WORKDIR/tap" push
