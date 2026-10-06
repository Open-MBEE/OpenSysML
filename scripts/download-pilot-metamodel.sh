#!/usr/bin/env bash
# Download the OMG SysML v2 metamodel, the pilot implementation's
# org.omg.sysml/model/SysML.ecore, into build/pilot-metamodel/, for
# tools/gen/ontology, which generates internal/translate/rdf/ontology/table.go
# from it and, with -check, fails when the committed table has drifted.
#
# The metamodel is not vendored: it belongs to the OMG pilot implementation and
# is licensed there. It is pinned by scripts/pilot-pin.sh like every other pilot
# input, so the table moves with the pilot release the rest of the tree is
# measured against. The download records that pin in a .pilot-pin stamp, which
# the generator writes into the table's header, and is re-fetched when the stamp
# does not match.
set -euo pipefail

# shellcheck source=scripts/pilot-pin.sh
. "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/pilot-pin.sh"

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
target="$repo_root/build/pilot-metamodel"

PILOT_FETCH_GLOBS=('SysML.ecore')
pilot_fetch_subtrees "org.omg.sysml/model:$target"

echo "Regenerate or check the metamodel table against it with:"
echo "  go run -C tools ./gen/ontology [-check]"
