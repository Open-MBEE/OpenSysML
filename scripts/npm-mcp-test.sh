#!/usr/bin/env bash
# Proves that `npx @openmbee/sysml-mcp` starts the server: packs the shim package
# (packaging/npm/sysml-mcp) and the platform package for this machine as the
# release job does, installs both tarballs into a throwaway project, and speaks
# MCP to the installed `sysml-mcp` command over stdio (`initialize`, then
# `tools/list`). Nothing is published and no registry is contacted.
#
#   scripts/npm-mcp-test.sh [<sysml-mcp binary>]
#
# The binary defaults to bin/sysml-mcp (built with `make build-mcp`). With
# SYSML_MCP_PACKAGES=<dir> the platform packages the generator already built
# there (the release job's client/node/packages) are packed instead of a set
# generated from the binary.
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
binary=${1:-"$root/bin/sysml-mcp"}
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

case "$(uname -s)" in
Linux) host_os=linux ;;
Darwin) host_os=darwin ;;
*) echo "npm-mcp-test: unsupported host $(uname -s)" >&2 && exit 1 ;;
esac
case "$(uname -m)" in
x86_64 | amd64) host_cpu=x64 && host_arch=amd64 ;;
aarch64 | arm64) host_cpu=arm64 && host_arch=arm64 ;;
*) echo "npm-mcp-test: unsupported host $(uname -m)" >&2 && exit 1 ;;
esac

sha256() {
	if command -v sha256sum >/dev/null 2>&1; then sha256sum "$@"; else shasum -a 256 "$@"; fi
}

shim="$root/packaging/npm/sysml-mcp"
version=$(node -p "require('$shim/package.json').version")
packages=${SYSML_MCP_PACKAGES:-}
if [[ -z "$packages" ]]; then
	[[ -x "$binary" ]] || { echo "npm-mcp-test: $binary is not an executable; run make build-mcp" >&2; exit 1; }
	# The generator builds all five platforms or none, so the binary stands in
	# for each; only this machine's package is installed below.
	release="$work/release"
	mkdir -p "$release"
	for platform in linux-amd64 linux-arm64 darwin-amd64 darwin-arm64 windows-amd64.exe; do
		cp "$binary" "$release/sysml-mcp-$platform"
		(cd "$release" && sha256 "sysml-mcp-$platform" > "sysml-mcp-$platform.sha256")
	done
	packages="$work/packages"
	node "$root/client/node/scripts/build-platform-packages.mjs" --mcp "$release" --out "$packages" > /dev/null
fi
platform_dir="$packages/sysml-mcp-$host_os-$host_cpu"
[[ -d "$platform_dir" ]] || { echo "npm-mcp-test: $platform_dir was not generated" >&2; exit 1; }

# The shim's own tests: platform resolution, the missing-package error,
# exit status and signal forwarding.
(cd "$shim" && npm test)

# Packed exactly as published, then installed from the tarballs: the shim's
# optional dependency on the platform package is satisfied by the tarball
# beside it, at the same version, so nothing is fetched.
tarballs="$work/tarballs"
mkdir -p "$tarballs"
npm pack --silent --pack-destination "$tarballs" "$shim" "$platform_dir"
ls -l "$tarballs"
project="$work/project"
mkdir -p "$project"
(cd "$project" && npm init -y > /dev/null && npm install --silent --no-audit --no-fund \
	"$tarballs/openmbee-sysml-mcp-$version.tgz" "$tarballs/openmbee-sysml-mcp-$host_os-$host_cpu-$version.tgz")
installed="$project/node_modules/.bin/sysml-mcp"
[[ -x "$installed" ]] || { echo "npm-mcp-test: npm did not link the sysml-mcp command" >&2; exit 1; }

# The command the host would start, against a small workspace.
workspace="$work/model"
mkdir -p "$workspace"
printf 'package Probe {\n    part def Vehicle;\n}\n' > "$workspace/probe.sysml"
"$installed" -version
node "$root/scripts/mcp-stdio-probe.mjs" -- "$installed" -root "$workspace"

# Without the platform package, the shim names it on stderr and fails.
bare="$work/bare"
mkdir -p "$bare"
(cd "$bare" && npm init -y > /dev/null && npm install --silent --no-audit --no-fund --omit=optional \
	"$tarballs/openmbee-sysml-mcp-$version.tgz")
if output=$("$bare/node_modules/.bin/sysml-mcp" -version 2>&1 >/dev/null); then
	echo "npm-mcp-test: the shim ran without its platform package" >&2
	exit 1
fi
expected="@openmbee/sysml-mcp-$host_os-$host_cpu@$version is not installed"
if [[ "$output" != *"$expected"* ]]; then
	echo "npm-mcp-test: the shim's error does not name the platform package:" >&2
	echo "$output" >&2
	exit 1
fi
echo "npm-mcp-test: @openmbee/sysml-mcp@$version served initialize and tools/list on $host_os-$host_cpu ($host_arch)"
