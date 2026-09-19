#!/usr/bin/env bash
# Provision everything tools/referee/reject needs: the pinned batch SysML validator
# and the pinned KerML validator. The negative corpus itself is committed under
# tools/referee/reject/testdata/negative and needs no download.
set -euo pipefail

scripts_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

"$scripts_dir/download-pilot-sysml-validator.sh" "$@"
"$scripts_dir/download-pilot-kerml-validator.sh" "$@"
