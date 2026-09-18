#!/usr/bin/env bash
# Download the OMG pilot implementation's own Xpect test suites into
# build/pilot-xpect-corpus/. Each suite ships its .xt files, the /src dependency
# models they import and its own copy of the standard library, so the harness
# can load exactly the resource set an XPECT_SETUP block declares.
#
# Like the other corpora these are not vendored: they belong to the OMG pilot
# implementation and are licensed there. tools/referee/xpect skips a suite whose
# directory is absent, so this script is optional for building and testing.
#
# They live under build/ rather than examples/ deliberately: the .kerml and
# .sysml models the suites ship are inputs to this harness, not models this
# repository ships, and everything that walks examples/ would otherwise adopt
# them.
#
# The release is pinned in scripts/pilot-pin.sh, the same pin the corpora and the
# reference validators use, so the declared expectations and the observed
# behaviour always come from one release. Each suite records that pin in a
# .pilot-pin stamp and is re-downloaded when the stamp does not match.
set -euo pipefail

# shellcheck source=scripts/pilot-pin.sh
. "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/pilot-pin.sh"

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
parent="$repo_root/build/pilot-xpect-corpus"

PILOT_FETCH_GLOBS=('*.xt')
pilot_fetch_subtrees \
	"org.omg.kerml.xpect.tests:$parent/kerml" \
	"org.omg.sysml.xpect.tests:$parent/sysml"

echo "Total $(pilot_count_files "$parent") .xt file(s) (the pin declares 303 KerML + 126 SysML = 429)."
echo "Compare our behaviour against their declared expectations with:"
echo "  go run -C tools ./cmd/pilot-xpect"
