#!/bin/sh
# Prints the release tag a rehearsal stands in for: the version
# client/python/opensysml/_version.py declares, spelled as the tag spells it.
# _version.py is PEP 440; the tag is SemVer. sed only: the CI images may lack python.
set -eu

version=$(sed -n 's/^VERSION *= *"\(.*\)" *$/\1/p' client/python/opensysml/_version.py)
case "$version" in
  *a[0-9]*) semver="${version%%a*}-alpha.${version##*a}" ;;
  *b[0-9]*) semver="${version%%b*}-beta.${version##*b}" ;;
  *rc[0-9]*) semver="${version%%rc*}-rc.${version##*rc}" ;;
  *) semver="$version" ;;
esac
if ! echo "$semver" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+(-(alpha|beta|rc)\.[0-9]+)?$'; then
  echo "Error: VERSION '$version' is not a release or an a/b/rc" >&2
  echo "pre-release; there is no SemVer tag spelling for it." >&2
  exit 1
fi
echo "v${semver}"
