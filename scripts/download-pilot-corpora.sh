#!/usr/bin/env bash
# Download the OMG SysML v2 and KerML corpora the differential harness compares
# beyond the training corpus, into examples/pilot-corpora/.
#
# Like the training examples, these are not vendored: they belong to the OMG
# pilot implementation and are licensed there. The harness skips a root whose
# directory is absent, so this script is optional for building and testing.
#
# The tag and the shared fetch both live in scripts/pilot-pin.sh, the same pin the
# training corpus and the reference validator use, so a comparison is always
# against one release.
set -euo pipefail

# shellcheck source=scripts/pilot-pin.sh
. "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/pilot-pin.sh"

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
parent="$repo_root/examples/pilot-corpora"

pilot_fetch_subtrees \
	"sysml/src/examples:$parent/sysml-examples" \
	"sysml/src/validation:$parent/sysml-validation" \
	"kerml/src/examples:$parent/kerml-examples"

echo "Compare them against the pilot implementation with:"
echo "  go run -C tools ./cmd/pilot-diff"
