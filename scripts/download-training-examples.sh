#!/usr/bin/env bash
# Download the OMG SysML v2 training examples into examples/sysml-v2-training/.
#
# The corpus is not vendored: it belongs to the OMG pilot implementation and is
# licensed there. The tests that read it skip when it is absent, so this script
# is optional for building and running the suite — but `go test ./internal/workspace/model`
# only gates the corpus once it has been downloaded.
#
# The tag is pinned in scripts/pilot-pin.sh, which also holds the shared fetch
# used here and by download-pilot-corpora.sh, so the expected results in
# internal/workspace/model/testdata are reproducible against one pilot release.
set -euo pipefail

# shellcheck source=scripts/pilot-pin.sh
. "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/pilot-pin.sh"

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

pilot_fetch_subtrees "sysml/src/training:$repo_root/examples/sysml-v2-training"
