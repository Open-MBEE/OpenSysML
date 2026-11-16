#!/usr/bin/env bash
# Block until the base branch moves off <sha>, then print its new head and exit.
# Usage: ci/poll_develop.sh [--config PATH] <sha>
set -uo pipefail
here=$(cd "$(dirname "$0")" && pwd)
cfg=()
if [ "${1:-}" = --config ]; then cfg=(--config "$2"); shift 2; fi
prev=${1:?usage: poll_develop.sh [--config PATH] <sha>}
repo=$(python3 "$here/throttle_common.py" "${cfg[@]}" get repo) || exit 1
branch=$(python3 "$here/throttle_common.py" "${cfg[@]}" get base_branch) || exit 1
while true; do
  cur=$(git ls-remote "https://github.com/$repo" "refs/heads/$branch" | cut -f1)
  if [ -n "$cur" ] && [ "$cur" != "$prev" ]; then echo "MOVED $cur"; exit 0; fi
  sleep 60
done
