#!/usr/bin/env bash
# Regenerates editors/mdk/compat/openapi-classes-<release>.txt from the public OpenAPI Javadoc
# class index of a release (https://jdocs.nomagic.com/<release>/allclasses-index.html), one fully
# qualified class per line with nested classes in Javadoc's Outer.Inner form. OpenApiFloorTest
# checks the plugin's vendor references against these files.
set -euo pipefail
if [[ "$#" -eq 0 ]]; then
  echo "usage: $0 <release>... (e.g. 2024x 2026xRefresh1)" >&2
  exit 2
fi
root=$(cd "$(dirname "$0")/.." && pwd)
for release in "$@"; do
  out="$root/compat/openapi-classes-$release.txt"
  curl -fsSL --proto '=https' --proto-redir '=https' "https://jdocs.nomagic.com/$release/allclasses-index.html" |
    grep -oE 'href="(com|org)/[^"#]+\.html"' |
    sed -E 's#^href="##; s#\.html"$##; s#/#.#g' |
    LC_ALL=C sort -u > "$out.tmp"
  if [[ "$(wc -l < "$out.tmp")" -lt 1000 ]]; then
    echo "$release: class index looks truncated ($(wc -l < "$out.tmp") classes)" >&2
    rm -f "$out.tmp"
    exit 1
  fi
  mv "$out.tmp" "$out"
  echo "$release: $(wc -l < "$out") classes -> ${out#"$root/"}"
done
