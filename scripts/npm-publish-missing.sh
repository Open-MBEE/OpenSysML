#!/usr/bin/env bash
# Publishes each npm package given (a package directory or a packed tarball)
# under a dist-tag, skipping any whose name is already on the registry at the
# version it declares: a publish cannot be repeated, so a re-run of a release
# job that stopped half-way completes the set instead of failing on the first
# package that went up. Run: scripts/npm-publish-missing.sh <dist-tag> <package>...
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
