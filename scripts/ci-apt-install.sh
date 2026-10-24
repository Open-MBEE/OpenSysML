#!/usr/bin/env bash
# Installs packages with apt on a CI runner so that a stalled mirror fails the
# step within minutes instead of holding it until the job's limit cancels it.
#
# Usage: scripts/ci-apt-install.sh [apt-get install options] <package>...
# CI_APT_TIMEOUT_SECONDS bounds one apt-get update, CI_APT_FETCH_SECONDS one fetch of the
# archives, CI_APT_DEADLINE_SECONDS all fetching;
# CI_APT_ARCHIVES names a directory of .deb files to seed apt's cache from and refill.
set -euo pipefail

[[ $# -gt 0 ]] || { echo "usage: $0 [apt-get install options] <package>..." >&2; exit 2; }

# A fetch that sends nothing for 30 s is abandoned and retried by apt itself; each
# command is bounded too, so a trickling mirror cannot hold it either.
acquire=(-o Acquire::Retries=3 -o Acquire::http::Timeout=30 -o Acquire::https::Timeout=30)
budget=${CI_APT_TIMEOUT_SECONDS:-180}
fetch=${CI_APT_FETCH_SECONDS:-300}
# Fetching ends inside the steps' ten-minute timeout-minutes with time to spare for
# unpacking, so a slow update cannot starve the download of its attempts.
deadline=$((SECONDS + ${CI_APT_DEADLINE_SECONDS:-480}))

# GitHub's runners list their mirrors by priority; a mirror that failed an attempt is
# dropped from the list so the next attempt asks the next one rather than retrying it.
mirrors=/etc/apt/apt-mirrors.txt
demote_mirror() {
  [[ -f $mirrors ]] && (($(grep -c . "$mirrors") > 1)) || return 0
  echo "dropping mirror $(head -n 1 "$mirrors" | cut -f 1) for the next attempt" >&2
  sudo sed -i '1d' "$mirrors"
}

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
    demote_mirror
    [[ $attempt -lt 3 ]] && sleep 15
  done
  return 1
}

# Without Error-Mode=any a failed index fetch is a warning and update exits 0,
# so the stall would only surface later, in install, against stale lists.
run_apt "$budget" update -o APT::Update::Error-Mode=any
# Archives are fetched in bounded, resumed attempts (Octave is ~200 MB) so a slow mirror
# gives way to the next; dpkg, which must not be interrupted, then runs on local files.
# Archives kept from an earlier run spare the mirror; apt re-fetches any whose hash
# no longer matches the index, so a stale copy costs nothing but its download.
archives=/var/cache/apt/archives
if [[ -n ${CI_APT_ARCHIVES:-} ]] && compgen -G "$CI_APT_ARCHIVES/*.deb" >/dev/null; then
  sudo cp "$CI_APT_ARCHIVES"/*.deb "$archives"/
fi
run_apt "$fetch" install -y --download-only "$@"
sudo apt-get "${acquire[@]}" install -y "$@"
if [[ -n ${CI_APT_ARCHIVES:-} ]]; then
  mkdir -p "$CI_APT_ARCHIVES"
  sudo find "$archives" -maxdepth 1 -name '*.deb' -exec cp -t "$CI_APT_ARCHIVES" {} +
  sudo chown -R "$(id -u):$(id -g)" "$CI_APT_ARCHIVES"
fi
