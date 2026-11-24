#!/usr/bin/env bash
# Installs packages with apt on a CI runner so that a stalled mirror fails the
# step within minutes instead of holding it until the job's limit cancels it.
#
# Usage: scripts/ci-apt-install.sh [apt-get install options] <package>...
set -euo pipefail

usage() { echo "usage: $0 [apt-get install options] <package>..." >&2; exit 2; }
# The apt-get install options come first, one argument each; the packages follow.
options=()
while [[ $# -gt 0 && $1 == -* ]]; do
  options+=("$1")
  shift
done
[[ $# -gt 0 ]] || usage
packages=("$@")

# An image that runs as root (CircleCI's julia) may carry no sudo.
if ((EUID == 0)) && ! command -v sudo >/dev/null; then
  sudo() { "$@"; }
fi

# A fetch that sends nothing for 30 s is abandoned and retried by apt itself; each
# command is bounded too, so a trickling mirror cannot hold it either.
acquire=(-o Acquire::Retries=3 -o Acquire::http::Timeout=30 -o Acquire::https::Timeout=30)
# CI_APT_CACHE_DIR relocates apt's archive cache; the tests point it at a directory of their own.
archives=${CI_APT_CACHE_DIR:-/var/cache/apt/archives}
cache=(-o "Dir::Cache::Archives=$archives/")
# CI_APT_TIMEOUT_SECONDS bounds one apt-get update and CI_APT_FETCH_SECONDS one fetch of the archives.
budget=${CI_APT_TIMEOUT_SECONDS:-180}
fetch=${CI_APT_FETCH_SECONDS:-300}
# All fetching ends (CI_APT_DEADLINE_SECONDS) inside the steps' ten-minute timeout-minutes with time
# to spare for unpacking, so a slow update cannot starve the download of its attempts.
deadline=$((SECONDS + ${CI_APT_DEADLINE_SECONDS:-480}))

# GitHub's runners list their mirrors by priority (CI_APT_MIRRORS names the list); a mirror that
# failed an attempt is dropped from it so the next attempt asks the next one rather than retrying it.
mirrors=${CI_APT_MIRRORS:-/etc/apt/apt-mirrors.txt}
demote_mirror() {
  [[ -f $mirrors ]] && (($(grep -c . "$mirrors") > 1)) || return 0
  echo "dropping mirror $(head -n 1 "$mirrors" | cut -f 1) for the next attempt" >&2
  sudo sed -i '1d' "$mirrors"
}

# The last update's output, which names the indexes that failed.
update_log=$(mktemp)
trap 'rm -rf "$update_log"' EXIT

# repositories <marker>: the repository roots of the http(s) URIs on stdin, cut at /dists/ or /pool/.
repositories() {
  local marker=$1
  sed -n "s|^\(https\{0,1\}://.*\)/$marker/.*|\1|p" | sort -u
}

# True when every index the last update failed on belongs to a repository the install fetches
# nothing from (a PPA the image lists); an index it cannot place counts as needed.
only_unneeded_indexes_failed() {
  local failed uris needed
  failed=$(sed -n 's/^[EW]: Failed to fetch \([^ ]*\).*/\1/p' "$update_log")
  [[ -n $failed ]] || return 1
  grep -v '^https\{0,1\}://.*/dists/' <<<"$failed" | grep -q . && return 1
  uris=$(apt-get "${cache[@]}" install -y --print-uris "${options[@]}" "${packages[@]}" 2>/dev/null) || return 1
  uris=$(sed -n "s/^'\([^']*\)'.*/\1/p" <<<"$uris")
  grep -v '^https\{0,1\}://.*/pool/' <<<"$uris" | grep -q . && return 1
  needed=$(repositories pool <<<"$uris")
  [[ -z $(comm -12 <(repositories dists <<<"$failed") <(echo "$needed")) ]] || return 1
  echo "apt-get update failed only on indexes ${packages[*]} does not install from:" >&2
  repositories dists <<<"$failed" | sed 's/^/  /' >&2
}

# run_apt <per-attempt bound in seconds> <apt-get arguments>...
run_apt() {
  local bound=$1 verb=$2 attempt left status
  shift
  for attempt in 1 2 3; do
    left=$((deadline - SECONDS))
    if ((left <= 0)); then
      echo "apt-get $verb: no time left for attempt $attempt of 3" >&2
      return 1
    fi
    status=0
    sudo timeout --kill-after=10s $((left < bound ? left : bound)) env LC_ALL=C apt-get "${acquire[@]}" "${cache[@]}" "$@" 2>&1 |
      tee "$update_log" || status=$?
    ((status == 0)) && return 0
    echo "apt-get $verb failed (attempt $attempt of 3)" >&2
    # apt's own exit, not a timeout: the lists it could fetch are complete, so the failure can be judged.
    if [[ $verb == update && $status == 100 ]] && only_unneeded_indexes_failed; then
      return 0
    fi
    demote_mirror
    [[ $attempt -lt 3 ]] && sleep 15
  done
  return 1
}

# CI_APT_ARCHIVES holds the .deb set an earlier run installed from. Seeding apt's cache with it
# spares the mirror, which re-fetches only an archive whose hash no longer matches the index.
seeded=()
if [[ -n ${CI_APT_ARCHIVES:-} ]]; then
  mkdir -p "$CI_APT_ARCHIVES"
  mapfile -t seeded < <(find "$CI_APT_ARCHIVES" -maxdepth 1 -name '*.deb' | sort)
  if ((${#seeded[@]} > 0)); then
    sudo cp "${seeded[@]}" "$archives"/
  fi
fi

# Without Error-Mode=any a failed index fetch is a warning and update exits 0,
# so the stall would only surface later, in install, against stale lists.
fetch_online() {
  run_apt "$budget" update -o APT::Update::Error-Mode=any || return 1
  # Archives are fetched in bounded, resumed attempts (Octave is ~200 MB) so a slow mirror
  # gives way to the next; dpkg, which must not be interrupted, then runs on local files.
  run_apt "$fetch" install -y --download-only "${options[@]}" "${packages[@]}"
}

# Given the seeded files rather than names, apt installs those versions whatever the stale
# lists say and fails on a dependency the set lacks; a requested package it lacks is checked after.
install_offline() {
  echo "installing ${packages[*]} offline from the ${#seeded[@]} archives under $CI_APT_ARCHIVES" >&2
  sudo apt-get "${acquire[@]}" "${cache[@]}" install -y --no-download "${options[@]}" "${seeded[@]}"
  local package status missing=()
  for package in "${packages[@]}"; do
    status=$(dpkg-query -W -f='${db:Status-Status}' "${package%%[=/]*}" 2>/dev/null) || status=absent
    [[ $status == installed ]] || missing+=("$package")
  done
  if ((${#missing[@]} > 0)); then
    echo "error: the archives under $CI_APT_ARCHIVES do not install ${missing[*]}" >&2
    return 1
  fi
}

if ! fetch_online; then
  if ((${#seeded[@]} == 0)); then
    echo "error: no mirror served ${packages[*]}, and ${CI_APT_ARCHIVES:-CI_APT_ARCHIVES} holds no archives to install offline" >&2
    exit 1
  fi
  echo "no mirror served ${packages[*]}" >&2
  install_offline
  exit 0
fi

# Asked against an empty cache, --print-uris names every archive the install uses, seeded or
# fetched; those alone are kept, so an archive the index moved on from leaves the set.
kept=()
if [[ -n ${CI_APT_ARCHIVES:-} ]]; then
  empty=$(mktemp -d)
  trap 'rm -rf "$update_log" "$empty"' EXIT
  mkdir -p "$empty/partial"
  uris=$(apt-get "${acquire[@]}" -o "Dir::Cache::Archives=$empty/" install -y --print-uris \
    "${options[@]}" "${packages[@]}")
  mapfile -t kept < <(sed -n "s/^'[^']*' \([^ ]*\) .*/\1/p" <<<"$uris")
fi
sudo apt-get "${acquire[@]}" "${cache[@]}" install -y "${options[@]}" "${packages[@]}"
if [[ -n ${CI_APT_ARCHIVES:-} ]]; then
  find "$CI_APT_ARCHIVES" -maxdepth 1 -name '*.deb' -delete
  if ((${#kept[@]} > 0)); then
    sudo cp "${kept[@]/#/$archives/}" "$CI_APT_ARCHIVES"/
    sudo chown "$(id -u):$(id -g)" "${kept[@]/#/$CI_APT_ARCHIVES/}"
  fi
  echo "kept the ${#kept[@]} archives ${packages[*]} installs from under $CI_APT_ARCHIVES" >&2
fi
