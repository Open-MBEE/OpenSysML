#!/usr/bin/env bash
# Installs the pinned buf release binary on a CI runner. `go run buf@version` would
# resolve buf's whole module graph against the Go checksum database on every run.
#
# Usage: scripts/ci-install-buf.sh [destination]   (default /usr/local/bin/buf)
set -euo pipefail

# The Makefile's BUF_VERSION, and the SHA-256 of that release's buf-Linux-x86_64.
BUF_VERSION=v1.57.2
BUF_SHA256=644e6fa4a62afc2b0d425d3ac9538d2f512947c0f2e82fc95c41a5eab6729414

[[ $# -le 1 ]] || { echo "usage: $0 [destination]" >&2; exit 2; }
destination=${1:-/usr/local/bin/buf}

download=$(mktemp -t buf.XXXXXX)
trap 'rm -f "$download"' EXIT
curl -fsSL --proto '=https' --proto-redir '=https' --retry 3 -o "$download" \
  "https://github.com/bufbuild/buf/releases/download/$BUF_VERSION/buf-Linux-x86_64"
echo "$BUF_SHA256  $download" | sha256sum --check --strict

# A destination the user cannot write (/usr/local/bin on a hosted runner) is installed with sudo.
install=(install -m 0755)
[[ -w $(dirname "$destination") ]] || install=(sudo "${install[@]}")
"${install[@]}" "$download" "$destination"
"$destination" --version
