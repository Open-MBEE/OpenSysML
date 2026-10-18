#!/usr/bin/env bash
#
# Render the Homebrew formula for a published release, ready to commit to the
# Open-MBEE/homebrew-tap repository as Formula/opensysml.rb.
#
# Usage:
#   scripts/render-homebrew-formula.sh <tag> [SHA256SUMS.txt] > Formula/opensysml.rb
#
# <tag> is the release tag as it appears in the GitHub release URL (e.g. v0.3.0).
# If the checksum file is omitted it is downloaded from the release. The
# checksums are produced by the build-release job in .circleci/config.yml.
#
# The rendered formula is committed to the tap repository Open-MBEE/homebrew-tap,
# which is not part of this repository. See packaging/homebrew/README.md.
set -euo pipefail

TAG="${1:-}"
SUMS="${2:-}"

if [[ -z "$TAG" ]]; then
  echo "usage: $0 <tag> [SHA256SUMS.txt]" >&2
  exit 2
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEMPLATE="${SCRIPT_DIR}/../packaging/homebrew/Formula/opensysml.rb"

if [[ ! -f "$TEMPLATE" ]]; then
  echo "error: formula source not found at $TEMPLATE" >&2
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

DARWIN_ARM64="$(sum_for opensysml-darwin-arm64.tar.gz)"
DARWIN_AMD64="$(sum_for opensysml-darwin-amd64.tar.gz)"
LINUX_ARM64="$(sum_for opensysml-linux-arm64.tar.gz)"
LINUX_AMD64="$(sum_for opensysml-linux-amd64.tar.gz)"
MCP_DARWIN_ARM64="$(sum_for sysml-mcp-darwin-arm64.tar.gz)"
MCP_DARWIN_AMD64="$(sum_for sysml-mcp-darwin-amd64.tar.gz)"
MCP_LINUX_ARM64="$(sum_for sysml-mcp-linux-arm64.tar.gz)"
MCP_LINUX_AMD64="$(sum_for sysml-mcp-linux-amd64.tar.gz)"

sed \
  -e "s|__TAG__|${TAG}|g" \
  -e "s|__SHA256_DARWIN_ARM64__|${DARWIN_ARM64}|g" \
  -e "s|__SHA256_DARWIN_AMD64__|${DARWIN_AMD64}|g" \
  -e "s|__SHA256_LINUX_ARM64__|${LINUX_ARM64}|g" \
  -e "s|__SHA256_LINUX_AMD64__|${LINUX_AMD64}|g" \
  -e "s|__SHA256_MCP_DARWIN_ARM64__|${MCP_DARWIN_ARM64}|g" \
  -e "s|__SHA256_MCP_DARWIN_AMD64__|${MCP_DARWIN_AMD64}|g" \
  -e "s|__SHA256_MCP_LINUX_ARM64__|${MCP_LINUX_ARM64}|g" \
  -e "s|__SHA256_MCP_LINUX_AMD64__|${MCP_LINUX_AMD64}|g" \
  "$TEMPLATE" |
  awk 'started || /^class /{ started = 1; print }' > "$OUT" # drop the maintainer-facing header comment

if grep -q '__[A-Z0-9_]*__' "$OUT"; then
  echo "error: unsubstituted placeholders remain:" >&2
  grep -o '__[A-Z0-9_]*__' "$OUT" | sort -u >&2
  exit 1
fi

cat "$OUT"
