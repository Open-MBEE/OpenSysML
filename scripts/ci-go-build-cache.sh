#!/usr/bin/env bash

# Keeps a saved Go build cache to the entries one run used: `age` marks every restored entry
# unused before the run, and `prune` removes those still marked after it, refusing when none was marked.
set -euo pipefail

usage() { echo "usage: $0 age|prune" >&2; exit 2; }
[[ $# -eq 1 ]] || usage

cache=$(go env GOCACHE)
# The time `age` gave the entries; outside the cache, so it is never saved with it.
stamp=${CI_GO_BUILD_CACHE_STAMP:-${RUNNER_TEMP:-${TMPDIR:-/tmp}}/go-build-cache.aged}

# An entry is <xx>/<hash>-a or <xx>/<hash>-d, the latter a directory for a linked executable. Go sets
# the time of each entry it uses to now once that time is an hour old (cmd/go/internal/cache, markUsed).
entries() {
  [[ -d $cache ]] || return 0
  find "$cache" -mindepth 2 -maxdepth 2 \( -name '*-a' -o -name '*-d' \) "$@"
}

case $1 in
  age)
    touch -d '2 hours ago' "$stamp"
    entries -exec touch -r "$stamp" {} +
    echo "marked $(entries | wc -l) build cache entries unused"
    ;;
  prune)
    if [[ ! -e $stamp ]]; then
      echo "error: $stamp is absent: the entries were not marked before the run, so none is known to be unused" >&2
      exit 1
    fi
    before=$(entries | wc -l)
    entries ! -newer "$stamp" -exec rm -rf {} +
    rm -f "$stamp"
    echo "kept $(entries | wc -l) of $before build cache entries"
    ;;
  *) usage ;;
esac
