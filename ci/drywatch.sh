#!/usr/bin/env bash
# On each move of the base branch, run drycheck.sh and append its results to <state_dir>/dry.log.
# Usage: ci/drywatch.sh [--config PATH]
set -uo pipefail
here=$(cd "$(dirname "$0")" && pwd)
conf() { python3 "$here/throttle_common.py" "$@"; }
repo=$(conf "$@" get repo) || exit 1
state=$(conf "$@" get state_dir) || exit 1
branch=$(conf "$@" get base_branch) || exit 1
last=""
while true; do
  d=$(timeout 30 gh api "repos/$repo/commits/$branch" --jq .sha 2>/dev/null)
  if [ -n "$d" ] && [ "$d" != "$last" ]; then
    echo "$(date -u +%FT%TZ) $branch ${d:0:10}" >>"$state/dry.log"
    "$here/drycheck.sh" "$@" >>"$state/dry.log" 2>&1
    last=$d
  fi
  sleep 60
done
