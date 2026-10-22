#!/usr/bin/env bash
# Installs packages with apt on a CI runner so that a stalled mirror fails the
# step within minutes instead of holding it until the job's limit cancels it.
#
# Usage: scripts/ci-apt-install.sh [apt-get install options] <package>...
set -euo pipefail

[[ $# -gt 0 ]] || { echo "usage: $0 [apt-get install options] <package>..." >&2; exit 2; }

# A fetch that sends nothing for 30 s is abandoned and retried by apt itself; each
# command is bounded too, so a trickling mirror cannot hold it either.
acquire=(-o Acquire::Retries=3 -o Acquire::http::Timeout=30 -o Acquire::https::Timeout=30)
budget=${CI_APT_TIMEOUT:-180}
# The whole script ends inside the steps' ten-minute timeout-minutes, so a slow
# update cannot leave the install without time for its attempts and its report.
deadline=$((SECONDS + ${CI_APT_DEADLINE:-540}))

run_apt() {
  local attempt left
  for attempt in 1 2 3; do
    left=$((deadline - SECONDS))
    if ((left <= 0)); then
      echo "apt-get $1: no time left for attempt $attempt of 3" >&2
      return 1
    fi
    if sudo timeout --kill-after=10s $((left < budget ? left : budget)) apt-get "${acquire[@]}" "$@"; then
      return 0
    fi
    echo "apt-get $1 failed (attempt $attempt of 3)" >&2
    [[ $attempt -lt 3 ]] && sleep 15
  done
  return 1
}

# Without Error-Mode=any a failed index fetch is a warning and update exits 0,
# so the stall would only surface later, in install, against stale lists.
run_apt update -o APT::Update::Error-Mode=any
run_apt install -y "$@"
