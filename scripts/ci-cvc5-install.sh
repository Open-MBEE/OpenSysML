#!/usr/bin/env bash

# Installs the pinned cvc5 on a CI runner from its release zip, kept in CI_CVC5_CACHE, so a run
# that has the verified zip never asks GitHub and one that does not rides out a failed connection.
set -euo pipefail

repo_root=$(cd "$(dirname "$0")/.." && pwd)
# shellcheck source=scripts/cvc5-pin.sh
source "$repo_root/scripts/cvc5-pin.sh"

# --pin prints what names a cache entry of the zip: the version and the digest.
if [[ ${1-} == --pin ]]; then
  echo "version=$CVC5_VERSION"
  echo "sha256=$CVC5_SHA256"
  exit 0
fi
if [[ $# -ne 0 ]]; then
  echo "usage: [CI_CVC5_CACHE=<dir>] [CI_CVC5_BIN=<dir>] $0 [--pin]" >&2
  exit 2
fi

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
cache=${CI_CVC5_CACHE:-$work/cache}
bin=${CI_CVC5_BIN:-/usr/local/bin}
zip=$cache/$CVC5_FILE

# pinned <file>: the file is the release asset the pin names, by its sha256 alone.
pinned() {
  local file=$1
  echo "$CVC5_SHA256  $file" | sha256sum --check --strict --quiet >&2
}

if [[ -f $zip ]] && pinned "$zip"; then
  echo "$zip is the pinned cvc5 $CVC5_VERSION; GitHub is not asked"
else
  [[ ! -e $zip ]] || echo "$zip is not the pinned cvc5 $CVC5_VERSION; downloading it again" >&2
  rm -f "$zip"
  # A zip that is not the pinned one is a substituted asset, not a transient failure: never retried, never kept.
  if ! "$repo_root/scripts/fetch-https.sh" "$CVC5_URL" "$work/$CVC5_FILE"; then
    echo "error: $CVC5_URL was not served within the retries of scripts/fetch-https.sh; nothing was installed" >&2
    exit 1
  fi
  if ! pinned "$work/$CVC5_FILE"; then
    echo "error: $CVC5_URL is not the pinned cvc5 $CVC5_VERSION ($CVC5_SHA256); nothing was installed" >&2
    exit 1
  fi
  mkdir -p "$cache"
  mv "$work/$CVC5_FILE" "$zip"
  echo "cvc5 $CVC5_VERSION downloaded into $zip"
fi

unzip -p "$zip" '*/bin/cvc5' >"$work/cvc5"
if [[ -w $bin ]]; then
  install -m 0755 "$work/cvc5" "$bin/cvc5"
else
  sudo install -m 0755 "$work/cvc5" "$bin/cvc5"
fi
cvc5_version="$("$bin/cvc5" --version)"
echo "${cvc5_version%%$'\n'*}"
