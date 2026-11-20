#!/usr/bin/env bash
# Dry-merge each PR listed in <state_dir>/ready.txt (one number per line) onto its base branch head.
# Usage: ci/drycheck.sh [--config PATH]
set -uo pipefail
here=$(cd "$(dirname "$0")" && pwd)
conf() { python3 "$here/throttle_common.py" "$@"; }
repo=$(conf "$@" get repo) || exit 1
state=$(conf "$@" get state_dir) || exit 1
checkout=$(conf "$@" get checkout) || exit 1
cd "$checkout" || exit 1
[[ -f "$state/ready.txt" ]] || { echo "no $state/ready.txt"; exit 0; }
while read -r n; do
  [[ "$n" =~ ^[0-9]+$ ]] || continue
  j=$(timeout 30 gh pr view "$n" --repo "$repo" --json state,headRefOid,baseRefName --jq '"\(.state) \(.headRefOid) \(.baseRefName)"') || { echo "#$n UNREADABLE"; continue; }
  read -r st head base <<<"$j"
  [[ "$st" == OPEN ]] || { echo "#$n $st"; continue; }
  git fetch -q origin "+refs/heads/$base:refs/tmp/base-$n" "+pull/$n/head:refs/tmp/pr-$n" || { echo "#$n FETCH-FAIL"; continue; }
  if git merge-tree --write-tree "refs/tmp/base-$n" "refs/tmp/pr-$n" >/dev/null 2>&1; then r=CLEAN; else r=DIRTY; fi
  echo "#$n ${head:0:10} on $base@$(git rev-parse --short=10 "refs/tmp/base-$n") $r"
done <"$state/ready.txt"
