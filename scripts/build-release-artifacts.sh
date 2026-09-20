#!/usr/bin/env bash
#
# Cross-compile sysml, sysml-lsp and sysml-grpc for every released platform and
# lay them out as release assets, the way the CircleCI `build-release` job does.
#
# Usage: VERSION=<version> scripts/build-release-artifacts.sh [dist-dir]
#
# VERSION is stamped into every binary through the Makefile's ldflags and is
# required, because a binary reporting `dev` looks identical to a correct one
# on a release page. COMMIT defaults to the checked-out revision. The
# directory (default `dist`) is emptied first and ends up holding:
#
#   sysml-<os>-<arch>.tar.gz, sysml-lsp-<os>-<arch>.tar.gz   (.zip on Windows)
#   opensysml-<os>-<arch>.tar.gz                              (.zip on Windows)
#   grpc/sysml-grpc-<os>-<arch>[.exe] with a .sha256 sidecar
#   SHA256SUMS.txt
#
# Every binary is then checked for the version it should report: the host
# platform's builds by running them, the cross-compiled ones for the version
# string the ldflags wrote into them. The Linux builds are also checked to be
# statically linked, so no release depends on the builder's glibc.
set -euo pipefail

: "${VERSION:?VERSION must be set to the version the binaries report}"
DIST="${1:-dist}"
COMMIT="${COMMIT:-$(git rev-parse --short HEAD)}"
BUILD_TIME="${BUILD_TIME:-$(date -u '+%Y-%m-%d_%H:%M:%S')}"
GO_VERSION="${GO_VERSION:-$(go version | awk '{print $3}')}"
PLATFORMS=(linux-amd64 linux-arm64 darwin-amd64 darwin-arm64 windows-amd64)
MAN="$(pwd)/packaging/man/man1"
CHECK_STATIC="$(pwd)/scripts/check-static-binaries.sh"

build() { # <make target> <binary name> <platform> <destination>
  local target="$1" binary="$2" platform="$3" dest="$4"
  local goos="${platform%-*}" goarch="${platform#*-}"
  GOOS="$goos" GOARCH="$goarch" make "$target" \
    VERSION="$VERSION" COMMIT="$COMMIT" BUILD_TIME="$BUILD_TIME" GO_VERSION="$GO_VERSION"
  # The Makefile writes bin/<binary> whatever GOOS is; the .exe is added here.
  if [[ "$goos" == windows ]]; then
    mv "bin/${binary}" "${dest}.exe"
  else
    mv "bin/${binary}" "$dest"
  fi
}

rm -rf "$DIST"
mkdir -p "$DIST/grpc"

for platform in "${PLATFORMS[@]}"; do
  build build-sysml sysml "$platform" "$DIST/sysml-${platform}"
  build build-lsp sysml-lsp "$platform" "$DIST/sysml-lsp-${platform}"
  # opensysml downloads one raw sysml-grpc per platform and verifies it against
  # a .sha256 sidecar, so these are published unarchived.
  build build-grpc sysml-grpc "$platform" "$DIST/grpc/sysml-grpc-${platform}"
done

cd "$DIST"

"$CHECK_STATIC" sysml-linux-* sysml-lsp-linux-* grpc/sysml-grpc-linux-*

for binary in sysml-*; do
  if [[ "$binary" == *.exe ]]; then
    zip -q "${binary%.exe}.zip" "$binary"
  else
    tar czf "${binary}.tar.gz" "$binary"
  fi
done

# Bundles: both binaries under their plain names with their manual pages, the layout a
# Homebrew formula or a PATH install expects. The per-binary archives stay for old links.
for platform in "${PLATFORMS[@]}"; do
  stage="stage/${platform}"
  if [[ "$platform" == windows-* ]]; then
    mkdir -p "$stage"
    cp "sysml-${platform}.exe" "${stage}/sysml.exe"
    cp "sysml-lsp-${platform}.exe" "${stage}/sysml-lsp.exe"
    (cd "$stage" && zip -q "../../opensysml-${platform}.zip" sysml.exe sysml-lsp.exe)
  else
    mkdir -p "$stage/share/man/man1"
    cp "sysml-${platform}" "${stage}/sysml"
    cp "sysml-lsp-${platform}" "${stage}/sysml-lsp"
    cp "$MAN/sysml.1" "$MAN/sysml-lsp.1" "${stage}/share/man/man1/"
    tar czf "opensysml-${platform}.tar.gz" -C "$stage" sysml sysml-lsp share
  fi
done
rm -rf stage

# Checksums over every published archive and every raw gRPC binary; the
# per-file sidecar is the only checksum opensysml reads.
sha256sum ./*.tar.gz ./*.zip | sed 's|\./||' > SHA256SUMS.txt
(cd grpc && sha256sum sysml-grpc-* >> ../SHA256SUMS.txt)
(cd grpc && for f in sysml-grpc-*; do sha256sum "$f" > "$f.sha256"; done)
cat SHA256SUMS.txt

host="$(go env GOHOSTOS)-$(go env GOHOSTARCH)"
status=0
fail() {
  echo "Error: $1 does not carry the version ${VERSION} it is being published as: $2." >&2
  echo "The version ldflags (see the Makefile's LDFLAGS) did not reach this build." >&2
  status=1
}
for binary in sysml-* grpc/sysml-grpc-*; do
  if [[ "$binary" == *.tar.gz || "$binary" == *.zip || "$binary" == *.sha256 ]]; then
    continue
  fi
  if [[ "$binary" == *"-${host}" ]]; then
    reported="$("./$binary" --version 2>&1 | head -n 1)"
    case " $reported " in
      *" ${VERSION} "*) echo "ok: $binary reports ${VERSION}" ;;
      *) fail "$binary" "it reports '${reported}'" ;;
    esac
  elif grep -qa -- "${VERSION}" "$binary"; then
    echo "ok: $binary carries ${VERSION}"
  else
    fail "$binary" "the version string is absent from the binary"
  fi
done
exit $status
