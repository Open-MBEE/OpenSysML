#!/usr/bin/env bash
# Checks that every Linux (ELF) binary given is statically linked: no program
# interpreter and no shared-library dependencies. A binary linked against the
# builder's libc would fail to start on systems with an older glibc.
#
#   scripts/check-static-binaries.sh bin/sysml bin/sysml-lsp bin/sysml-grpc
#
# Mach-O and PE binaries are not ELF and are reported as skipped; Go links
# those statically whatever CGO_ENABLED says.
set -euo pipefail

if [[ $# -eq 0 ]]; then
  echo "usage: $0 <binary>..." >&2
  exit 2
fi

status=0
for binary in "$@"; do
  if [[ ! -f "$binary" ]]; then
    echo "Error: $binary does not exist" >&2
    status=1
    continue
  fi
  if [[ "$(head -c 4 "$binary" | tr -d '\0')" != $'\x7fELF' ]]; then
    echo "skip: $binary is not an ELF binary"
    continue
  fi
  # Only ELF inputs need readelf, so a Mach-O-only host is not asked for binutils.
  if ! command -v readelf >/dev/null 2>&1; then
    echo "Error: readelf (binutils) is required to check $binary" >&2
    exit 2
  fi
  # A readelf failure must not read as "no interpreter, no libraries".
  if ! headers="$(readelf -lW "$binary")" || ! dynamic="$(readelf -dW "$binary")"; then
    echo "Error: readelf could not read $binary" >&2
    status=1
    continue
  fi
  interp="$(grep -c '^\s*INTERP' <<<"$headers" || true)"
  needed="$(grep -c '(NEEDED)' <<<"$dynamic" || true)"
  if [[ "$interp" == 0 && "$needed" == 0 ]]; then
    echo "ok: $binary is statically linked"
  else
    echo "Error: $binary is dynamically linked (interpreter: $interp, shared libraries: $needed);" >&2
    echo "it will require the builder's glibc. Build it with CGO_ENABLED=0 (the Makefile's GO_BUILD)." >&2
    status=1
  fi
done
exit $status
