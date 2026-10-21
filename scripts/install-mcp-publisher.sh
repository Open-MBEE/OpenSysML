#!/usr/bin/env bash
#
# Install the pinned mcp-publisher release (the MCP Registry's publishing CLI)
# into a directory, verifying the archive against its published SHA-256.
#
# Usage: scripts/install-mcp-publisher.sh [DEST_DIR]   (default build/mcp-publisher)
# Prints the path of the installed binary. A binary of the pinned version that
# is already there is kept.
set -euo pipefail

MCP_PUBLISHER_VERSION="1.8.1"
# sha256 of mcp-publisher_<os>_<arch>.tar.gz from the release's registry_<version>_checksums.txt.
declare -A MCP_PUBLISHER_SHA256=(
  [linux_amd64]="a06c9096dcb9727c13555b6be26c7effa707b01f06a4c561ba7a3635443cf2cc"
  [linux_arm64]="8dd75a6cf6845688b5d4e46df58d3ca26d5c8d233bb0626606e1db82c5e883e4"
  [darwin_amd64]="88126981225e7714fcc6b7a10cdba4a80ae5901e9740a8c06d0d5195c8bc294c"
  [darwin_arm64]="e45e520892460732a4bdf37255576415d4a53ec171f8b913faf15bb1aef7cb77"
)

dest="${1:-build/mcp-publisher}"
binary="$dest/mcp-publisher"

if [[ -x "$binary" ]] && "$binary" --version 2>/dev/null | grep -q "$MCP_PUBLISHER_VERSION"; then
  echo "$binary"
  exit 0
fi

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"
case "$arch" in
  x86_64) arch=amd64 ;;
  aarch64 | arm64) arch=arm64 ;;
esac
platform="${os}_${arch}"
expected="${MCP_PUBLISHER_SHA256[$platform]:-}"
if [[ -z "$expected" ]]; then
  echo "Error: no pinned mcp-publisher checksum for $platform; add it from the release's checksums file." >&2
  exit 1
fi

archive="mcp-publisher_${platform}.tar.gz"
url="https://github.com/modelcontextprotocol/registry/releases/download/v${MCP_PUBLISHER_VERSION}/${archive}"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
curl --proto '=https' --tlsv1.2 -fsSL --retry 3 -o "$tmp/$archive" "$url"
if command -v sha256sum >/dev/null 2>&1; then
  actual="$(sha256sum "$tmp/$archive" | cut -d' ' -f1)"
else
  actual="$(shasum -a 256 "$tmp/$archive" | cut -d' ' -f1)"
fi
if [[ "$actual" != "$expected" ]]; then
  echo "Error: $archive has sha256 $actual, not the pinned $expected; refusing to install it." >&2
  exit 1
fi
tar -xzf "$tmp/$archive" -C "$tmp" mcp-publisher
mkdir -p "$dest"
install -m 0755 "$tmp/mcp-publisher" "$binary"
echo "$binary"
