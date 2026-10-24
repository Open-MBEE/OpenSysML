#!/usr/bin/env bash
#
# Render the Scoop manifest for a published release, ready to submit to a
# Scoop bucket as bucket/opensysml.json.
#
# Usage:
#   scripts/render-scoop-manifest.sh <tag> [SHA256SUMS.txt] > opensysml.json
#
# <tag> is the release tag as it appears in the GitHub release URL (e.g. v0.3.0).
# If the checksum file is omitted it is downloaded from the release. The
# checksums are produced by the build-release job in .circleci/config.yml.
#
# The manifest is submitted to an external bucket by a maintainer, by hand;
# nothing here writes anywhere. See packaging/scoop/README.md.
set -euo pipefail

TAG="${1:-}"
SUMS="${2:-}"

if [[ -z "$TAG" ]]; then
  echo "usage: $0 <tag> [SHA256SUMS.txt]" >&2
  exit 2
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEMPLATE="${SCRIPT_DIR}/../packaging/scoop/opensysml.json"

if [[ ! -f "$TEMPLATE" ]]; then
  echo "error: manifest template not found at $TEMPLATE" >&2
  exit 1
fi

OUT="$(mktemp)"
cleanup() { rm -f "$OUT" "${TMP_SUMS:-}"; }
trap cleanup EXIT

if [[ -z "$SUMS" ]]; then
  TMP_SUMS="$(mktemp)"
  URL="https://github.com/Open-MBEE/OpenSysML/releases/download/${TAG}/SHA256SUMS.txt"
  echo "Fetching ${URL}" >&2
  curl -fsSL --proto '=https' --proto-redir '=https' "$URL" -o "$TMP_SUMS"
  SUMS="$TMP_SUMS"
fi

sum_for() {
  local archive="$1" sum
  sum="$(awk -v a="$archive" '$2 == a { print $1 }' "$SUMS")"
  if [[ -z "$sum" ]]; then
    echo "error: no checksum for $archive in $SUMS" >&2
    exit 1
  fi
  printf '%s' "$sum"
}

# Scoop versions carry no "v"; checkver's github source strips it the same way.
VERSION="${TAG#v}"
WINDOWS_AMD64="$(sum_for opensysml-windows-amd64.zip)"
MCP_WINDOWS_AMD64="$(sum_for sysml-mcp-windows-amd64.zip)"

sed \
  -e "s|__TAG__|${TAG}|g" \
  -e "s|__VERSION__|${VERSION}|g" \
  -e "s|__SHA256_WINDOWS_AMD64__|${WINDOWS_AMD64}|g" \
  -e "s|__SHA256_MCP_WINDOWS_AMD64__|${MCP_WINDOWS_AMD64}|g" \
  "$TEMPLATE" |
  grep -v '^    "##": ' > "$OUT" # drop the maintainer-facing template note

if grep -q '__[A-Z0-9_]*__' "$OUT"; then
  echo "error: unsubstituted placeholders remain:" >&2
  grep -o '__[A-Z0-9_]*__' "$OUT" | sort -u >&2
  exit 1
fi

cat "$OUT"
