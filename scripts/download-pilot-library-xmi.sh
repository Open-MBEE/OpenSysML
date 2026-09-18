#!/usr/bin/env bash
# Download the pilot's XMI serialization of the standard library into
# build/pilot-library-xmi/, for the normative identity gate in
# tests/identity (TestPilotLibraryXMI).
#
# The XMI carries the element ids the pilot fixes for every standard-library
# element; the gate asserts the ids this implementation derives are the same
# ones, element for element and owning membership for owning membership. It is
# not vendored: it belongs to the OMG pilot implementation and is licensed
# there. The gate skips while the directory is absent, so this script is
# optional for building and testing; CI sets OPENSYSML_REQUIRE_PILOT_LIBRARY_XMI=1
# so an absent copy fails there.
#
# The library is published in the pilot's release repository under the same tag
# as the pilot itself, so the release is pinned in scripts/pilot-pin.sh next to
# the pilot's, and the bundled library under internal/core/libs/stdlib is the
# notation from the same commit. The download records that pin in a .pilot-pin
# stamp and is re-fetched when the stamp does not match.
set -euo pipefail

# shellcheck source=scripts/pilot-pin.sh
. "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/pilot-pin.sh"

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
target="$repo_root/build/pilot-library-xmi"

pilot_from_release
PILOT_FETCH_GLOBS=('*.sysmlx' '*.kermlx')
pilot_fetch_subtrees "sysml.library.xmi:$target"

echo "Total $(pilot_count_files "$target") XMI file(s)."
echo "Compare the ids this implementation derives against them with:"
echo "  go test -count=1 ./tests/identity -run TestPilotLibraryXMI"
