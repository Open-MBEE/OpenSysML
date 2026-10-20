#!/usr/bin/env bash
# Publishes each package (directory or tarball) under a dist-tag, skipping any the
# registry already has at its version, so a re-run completes a half-published set.
set -euo pipefail

if [[ $# -lt 2 ]]; then
	echo "usage: $0 <dist-tag> <package-dir-or-tarball>..." >&2
	exit 2
fi
tag=$1
shift

manifest() {
	if [[ -d "$1" ]]; then cat "$1/package.json"; else tar -xOf "$1" package/package.json; fi
}

for package in "$@"; do
	read -r name version < <(manifest "$package" | node -p '
		const pkg = JSON.parse(require("fs").readFileSync(0, "utf8"));
		pkg.name + " " + pkg.version')
	if npm view "${name}@${version}" version > /dev/null 2>&1; then
		echo "${name}@${version} is already on the registry; skipping it."
		continue
	fi
	echo "Publishing ${name}@${version} to the '${tag}' dist-tag from ${package}"
	npm publish "$package" --access public --tag "$tag"
done
