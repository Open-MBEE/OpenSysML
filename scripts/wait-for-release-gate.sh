#!/usr/bin/env bash
#
# Wait until CircleCI has published the gated GitHub release for a tag.
#
# Usage: scripts/wait-for-release-gate.sh <tag> <commit-sha>
# Needs `gh` authenticated (GH_TOKEN) and GH_REPO set to owner/name.
#
# CircleCI publishes SHA256SUMS.txt.bundle last, and only after the test suite
# and build-release passed on the tag, so its presence is the release gate:
# nothing is signed or added to a release CircleCI rejected. The tag must also
# still resolve to the commit this workflow built.
set -euo pipefail

TAG="${1:?usage: $0 <tag> <commit-sha>}"
COMMIT="${2:?usage: $0 <tag> <commit-sha>}"
: "${GH_REPO:?GH_REPO must be set to owner/name}"
TIMEOUT="${RELEASE_GATE_TIMEOUT:-5400}"

deadline=$((SECONDS + TIMEOUT))
until gh release view "$TAG" --json assets --jq '.assets[].name' 2>/dev/null | grep -qx 'SHA256SUMS.txt.bundle'; do
  if [[ "$SECONDS" -ge "$deadline" ]]; then
    echo "Error: CircleCI has not published SHA256SUMS.txt.bundle on release $TAG; giving up." >&2
    exit 1
  fi
  echo "Waiting for CircleCI to publish release $TAG..."
  sleep 60
done
resolved=$(gh api "repos/$GH_REPO/commits/$TAG" --jq .sha)
if [[ "$resolved" != "$COMMIT" ]]; then
  echo "Error: tag $TAG resolves to $resolved, this workflow built $COMMIT." >&2
  exit 1
fi
echo "Release $TAG is published by CircleCI and matches $COMMIT."
