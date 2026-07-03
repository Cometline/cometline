#!/usr/bin/env bash
set -euo pipefail

TAG="${1:?Usage: render-homebrew-cask.sh <tag> <sha256> [download_repo]}"
SHA="${2:?Usage: render-homebrew-cask.sh <tag> <sha256> [download_repo]}"
DOWNLOAD_REPO="${3:-cometline/cometline}"

VERSION="${TAG#v}"
if [ "$VERSION" = "$TAG" ]; then
  echo "tag must start with v (got: $TAG)" >&2
  exit 1
fi

cat <<EOF
cask "cometline" do
  version "${VERSION}"
  sha256 "${SHA}"

  url "https://github.com/${DOWNLOAD_REPO}/releases/download/v#{version}/Cometline-#{version}-mac.zip"
  name "Cometline"
  desc "Local-first AI companion for your workspace"
  homepage "https://github.com/${DOWNLOAD_REPO}"
  auto_updates true
  depends_on macos: ">= :ventura"

  app "Cometline.app"
end
EOF
