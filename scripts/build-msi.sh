#!/usr/bin/env bash
#
# Build the Windows installer (per-machine x64 MSI) from cross-built binaries,
# on Linux, with the WiX v5 .NET tool.
#
# Usage:
#   scripts/build-msi.sh <tag> <sysml.exe> <sysml-lsp.exe> <sysml-grpc.exe> [out.msi]
#
# <tag> is the release tag (v0.5.0, v0.6.0-rc1). The MSI ProductVersion is its
# numeric x.y.z part; the full tag is kept in the OPENSYSML_TAG property and
# the Add/Remove Programs comment. The default output is
# dist/opensysml-<x.y.z>-windows-amd64.msi.
#
# The optional "SMT solver (Z3)" feature bundles the Z3 release pinned in
# packaging/msi/z3.pin: the zip is downloaded (or read from $Z3_ZIP_CACHE) and
# refused unless its SHA256 matches the pin.
#
# Requires: the WiX 5 .NET tool (`dotnet tool install --global wix --version 5.0.2`),
# curl, unzip (or 7z), sha256sum. No WiX extensions are used.
# WiX only runs on Windows, so CI runs this on a windows runner (Git Bash). For a
# Linux dry run set WIX_CMD to a command that runs it under Wine and
# WIX_PATH_PREFIX to Wine's drive for `/` (usually Z:); see packaging/msi/README.md.
# (The override is deliberately not named WIX: the WiX v3 installer, present on
# the GitHub Windows runners, exports WIX as its installation directory.)
set -euo pipefail

WIX_CMD="${WIX_CMD:-wix}"
WIX_PATH_PREFIX="${WIX_PATH_PREFIX:-}"
# Absolute host path -> the path as the (Windows) wix process sees it.
if command -v cygpath >/dev/null 2>&1; then
  wixpath() { local host="$1"; cygpath -m "$host"; }  # Git Bash / MSYS2: /c/x -> C:/x
else
  wixpath() { local host="$1"; printf '%s%s' "$WIX_PATH_PREFIX" "$host"; }
fi

if [[ $# -lt 4 || $# -gt 5 ]]; then
  echo "usage: $0 <tag> <sysml.exe> <sysml-lsp.exe> <sysml-grpc.exe> [out.msi]" >&2
  exit 2
fi

TAG="$1"
SYSML_EXE="$2"
LSP_EXE="$3"
GRPC_EXE="$4"
OUT="${5:-}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
MSI_DIR="$REPO_ROOT/packaging/msi"
WXS="$MSI_DIR/opensysml.wxs"
PIN="$MSI_DIR/z3.pin"
LICENSE_FILE="$REPO_ROOT/LICENSE"

for f in "$WXS" "$PIN" "$LICENSE_FILE"; do
  [[ -f "$f" ]] || { echo "error: $f not found" >&2; exit 1; }
done
for f in "$SYSML_EXE" "$LSP_EXE" "$GRPC_EXE"; do
  [[ -f "$f" ]] || { echo "error: binary $f not found" >&2; exit 1; }
done
for tool in ${WIX_CMD%% *} curl sha256sum; do
  command -v "$tool" >/dev/null 2>&1 || { echo "error: $tool is required" >&2; exit 1; }
done
# Git for Windows ships no unzip; the GitHub runners have 7z.
if command -v unzip >/dev/null 2>&1; then
  extract() { local archive="$1" dest="$2"; unzip -q -j "$archive" "${@:3}" -d "$dest"; }
elif command -v 7z >/dev/null 2>&1; then
  extract() { local archive="$1" dest="$2"; 7z e -y -bso0 -bsp0 "-o$dest" "$archive" "${@:3}" >/dev/null; }
else
  echo "error: unzip or 7z is required" >&2; exit 1
fi

# v0.5.0 -> 0.5.0; v0.6.0-rc1 -> 0.6.0. MSI ProductVersion must be numeric
# x.y.z with x,y < 256 and z < 65536.
case "$TAG" in
  v[0-9]*.[0-9]*.[0-9]*) ;;
  *) echo "error: tag '$TAG' must look like v<major>.<minor>.<patch>[-suffix]" >&2; exit 1 ;;
esac
VERSION="${TAG#v}"
VERSION="${VERSION%%-*}"
VERSION="${VERSION%%+*}"
IFS=. read -r MAJOR MINOR PATCH <<<"$VERSION"
if [[ ! "$MAJOR$MINOR$PATCH" =~ ^[0-9]+$ ]] || (( MAJOR > 255 || MINOR > 255 || PATCH > 65535 )); then
  echo "error: '$VERSION' is not a valid MSI ProductVersion (x<256.y<256.z<65536)" >&2
  exit 1
fi

if [[ -z "$OUT" ]]; then
  OUT="$REPO_ROOT/dist/opensysml-${VERSION}-windows-amd64.msi"
elif [[ "$OUT" != /* ]]; then
  OUT="$PWD/$OUT"
fi

# shellcheck source=packaging/msi/z3.pin
source "$PIN"
: "${Z3_VERSION:?z3.pin must set Z3_VERSION}" "${Z3_ZIP:?z3.pin must set Z3_ZIP}" "${Z3_SHA256:?z3.pin must set Z3_SHA256}"

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
mkdir -p "$WORK/bin" "$WORK/z3"

# The .wxs names the files as they are installed; the release assets carry
# platform suffixes, so stage them under their installed names.
cp "$SYSML_EXE" "$WORK/bin/sysml.exe"
cp "$LSP_EXE" "$WORK/bin/sysml-lsp.exe"
cp "$GRPC_EXE" "$WORK/bin/sysml-grpc.exe"

# Z3: fetch the pinned zip, verify it, take z3.exe, its VC++ runtime and the MIT notice.
Z3_CACHE_DIR="${Z3_ZIP_CACHE:-$WORK}"
Z3_ZIP_PATH="$Z3_CACHE_DIR/$Z3_ZIP"
if [[ ! -f "$Z3_ZIP_PATH" ]]; then
  Z3_URL="https://github.com/Z3Prover/z3/releases/download/z3-${Z3_VERSION}/${Z3_ZIP}"
  echo "Fetching $Z3_URL" >&2
  mkdir -p "$Z3_CACHE_DIR"
  curl -fsSL --proto '=https' --proto-redir '=https' "$Z3_URL" -o "$Z3_ZIP_PATH"
fi
ACTUAL="$(sha256sum "$Z3_ZIP_PATH" | awk '{print $1}')"
if [[ "$ACTUAL" != "$Z3_SHA256" ]]; then
  echo "error: $Z3_ZIP SHA256 is $ACTUAL, packaging/msi/z3.pin expects $Z3_SHA256" >&2
  rm -f "$Z3_ZIP_PATH"
  exit 1
fi
Z3_TOP="${Z3_ZIP%.zip}"
extract "$Z3_ZIP_PATH" "$WORK/z3" \
  "$Z3_TOP/bin/z3.exe" "$Z3_TOP/bin/msvcp140.dll" "$Z3_TOP/bin/vcruntime140.dll" \
  "$Z3_TOP/bin/vcruntime140_1.dll" "$Z3_TOP/LICENSE.txt"
mv "$WORK/z3/LICENSE.txt" "$WORK/z3/LICENSE-z3.txt"
for f in z3.exe msvcp140.dll vcruntime140.dll vcruntime140_1.dll LICENSE-z3.txt; do
  [[ -s "$WORK/z3/$f" ]] || { echo "error: $f missing from $Z3_ZIP" >&2; exit 1; }
done

mkdir -p "$(dirname "$OUT")"
echo "Building $OUT (ProductVersion $VERSION, tag $TAG, Z3 $Z3_VERSION)" >&2
# shellcheck disable=SC2086  # WIX_CMD may be a multi-word command (e.g. wine ... wix.dll)
$WIX_CMD build -arch x64 \
  -d "Version=$VERSION" \
  -d "Tag=$TAG" \
  -d "Z3Version=$Z3_VERSION" \
  -d "BinDir=$(wixpath "$WORK/bin")" \
  -d "Z3Dir=$(wixpath "$WORK/z3")" \
  -d "LicenseFile=$(wixpath "$LICENSE_FILE")" \
  -pdbtype none \
  -o "$(wixpath "$OUT")" \
  "$(wixpath "$WXS")"

[[ -s "$OUT" ]] || { echo "error: wix did not produce $OUT" >&2; exit 1; }

echo "Built $OUT" >&2
