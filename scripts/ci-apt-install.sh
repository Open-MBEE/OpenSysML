#!/usr/bin/env bash
# Installs packages with apt on a CI runner so that a stalled mirror fails the
# step within minutes instead of holding it until the job's limit cancels it.
#
# Usage: scripts/ci-apt-install.sh [apt-get install options] <package>...
# CI_APT_TIMEOUT_SECONDS bounds one apt-get update, CI_APT_DEADLINE_SECONDS all fetching.
set -euo pipefail

[[ $# -gt 0 ]] || { echo "usage: $0 [apt-get install options] <package>..." >&2; exit 2; }

# A fetch that sends nothing for 30 s is abandoned and retried by apt itself; each
# command is bounded too, so a trickling mirror cannot hold it either.
acquire=(-o Acquire::Retries=3 -o Acquire::http::Timeout=30 -o Acquire::https::Timeout=30)
budget=${CI_APT_TIMEOUT_SECONDS:-180}
# Fetching ends inside the steps' ten-minute timeout-minutes with time to spare for
# unpacking, so a slow update cannot starve the download of its attempts.
deadline=$((SECONDS + ${CI_APT_DEADLINE_SECONDS:-480}))

# run_apt <per-attempt bound in seconds> <apt-get arguments>...
run_apt() {
  local bound=$1 attempt left
  shift
  for attempt in 1 2 3; do
    left=$((deadline - SECONDS))
    if ((left <= 0)); then
      echo "apt-get $1: no time left for attempt $attempt of 3" >&2
      return 1
    fi
    if sudo timeout --kill-after=10s $((left < bound ? left : bound)) apt-get "${acquire[@]}" "$@"; then
      return 0
    fi
    echo "apt-get $1 failed (attempt $attempt of 3)" >&2
    [[ $attempt -lt 3 ]] && sleep 15
  done
  return 1
}

# Without Error-Mode=any a failed index fetch is a warning and update exits 0,
# so the stall would only surface later, in install, against stale lists.
run_apt "$budget" update -o APT::Update::Error-Mode=any
# The archives are fetched on their own, bounded by the deadline rather than the
# update budget (Octave is ~200 MB) and resumed by the next attempt, so that dpkg,
# which must not be interrupted, runs below with everything already local.
run_apt "$deadline" install -y --download-only "$@"
sudo apt-get "${acquire[@]}" install -y "$@"
