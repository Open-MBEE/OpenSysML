#!/usr/bin/env bash
#
# Render the conda-forge recipe for a published release, ready to submit to
# conda-forge/staged-recipes as recipes/jupyter-opensysml-kernel/meta.yaml.
#
# Usage:
#   scripts/render-conda-recipe.sh <tag> [SHA256SUMS.txt] > meta.yaml
#
# <tag> is the release tag as it appears in the GitHub release URL (e.g. v0.9.2).
# If the checksum file is omitted it is downloaded from the release. The
# checksums are produced by the build-release job in .circleci/config.yml.
# See packaging/conda/README.md.
set -euo pipefail

TAG="${1:-}"
SUMS="${2:-}"

if [[ -z "$TAG" ]]; then
  echo "usage: $0 <tag> [SHA256SUMS.txt]" >&2
  exit 2
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEMPLATE="${SCRIPT_DIR}/../packaging/conda/recipe/meta.yaml"

if [[ ! -f "$TEMPLATE" ]]; then
  echo "error: recipe source not found at $TEMPLATE" >&2
  exit 1
fi

cleanup() { rm -f "${TMP_SUMS:-}"; }
trap cleanup EXIT

if [[ -z "$SUMS" ]]; then
  TMP_SUMS="$(mktemp)"
  URL="https://github.com/Open-MBEE/OpenSysML/releases/download/${TAG}/SHA256SUMS.txt"
  echo "Fetching ${URL}" >&2
  curl -fsSL --proto '=https' --proto-redir '=https' "$URL" -o "$TMP_SUMS"
  SUMS="$TMP_SUMS"
fi

sum_for() {
  local asset="$1" sum
  sum="$(awk -v a="$asset" '{ n = $2; sub(/^\*/, "", n) } n == a { print $1 }' "$SUMS")"
  if [[ -z "$sum" ]]; then
    echo "error: no checksum for $asset in $SUMS" >&2
    exit 1
  fi
  printf '%s' "$sum"
}

# The sdist is named by the PEP 440 version; the manifest lists exactly one.
SDIST="$(awk '{ n = $2; sub(/^\*/, "", n) } n ~ /^jupyter_opensysml_kernel-[0-9].*\.tar\.gz$/ { print n }' "$SUMS")"
if [[ -z "$SDIST" || "$SDIST" == *$'\n'* ]]; then
  echo "error: $SUMS does not list exactly one jupyter_opensysml_kernel sdist" >&2
  exit 1
fi
VERSION="${SDIST#jupyter_opensysml_kernel-}"
VERSION="${VERSION%.tar.gz}"

sed \
  -e "s|__TAG__|${TAG}|g" \
  -e "s|__VERSION__|${VERSION}|g" \
  -e "s|__SHA256_SDIST__|$(sum_for "$SDIST")|g" \
  -e "s|__SHA256_LINUX_AMD64__|$(sum_for sysml-jupyter-kernel-linux-amd64)|g" \
  -e "s|__SHA256_LINUX_ARM64__|$(sum_for sysml-jupyter-kernel-linux-arm64)|g" \
  -e "s|__SHA256_DARWIN_AMD64__|$(sum_for sysml-jupyter-kernel-darwin-amd64)|g" \
  -e "s|__SHA256_DARWIN_ARM64__|$(sum_for sysml-jupyter-kernel-darwin-arm64)|g" \
  -e "s|__SHA256_WINDOWS_AMD64__|$(sum_for sysml-jupyter-kernel-windows-amd64.exe)|g" \
  "$TEMPLATE" | awk 'NR == 1 && /^# Source of the conda-forge recipe/ { skip = 1 } skip && /^#/ { next } { skip = 0; print }'
