#!/usr/bin/env bash
# Fetch the normative SysML.json (KerML 10.4.1) into build/omg-json-schema/, verified against the pinned SHA-256.
set -euo pipefail

OMG_SYSML_JSON_SCHEMA_URL="${OMG_SYSML_JSON_SCHEMA_URL:-https://www.omg.org/spec/SysML/20250201/SysML.json}"
OMG_SYSML_JSON_SCHEMA_SHA256="${OMG_SYSML_JSON_SCHEMA_SHA256:-bb0d8af159cf2cbe4a0df4ed6b903505a57e33d047068e5a50a7008a18d546c5}"

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
target_dir="$repo_root/build/omg-json-schema"
target="$target_dir/SysML.json"

sha256_of() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | cut -d' ' -f1
	else
		shasum -a 256 "$1" | cut -d' ' -f1
	fi
	return 0
}

if [[ -f "$target" ]] && [[ "$(sha256_of "$target")" == "$OMG_SYSML_JSON_SCHEMA_SHA256" ]]; then
	echo "Already present at $target (sha256 $OMG_SYSML_JSON_SCHEMA_SHA256)"
	exit 0
fi

mkdir -p "$target_dir"
work="$(mktemp "$target_dir/SysML.json.XXXXXX")"
trap 'rm -f "$work"' EXIT

echo "Fetching $OMG_SYSML_JSON_SCHEMA_URL ..."
curl --fail --silent --show-error --location --retry 3 --output "$work" "$OMG_SYSML_JSON_SCHEMA_URL"

actual="$(sha256_of "$work")"
if [[ "$actual" != "$OMG_SYSML_JSON_SCHEMA_SHA256" ]]; then
	echo "error: $OMG_SYSML_JSON_SCHEMA_URL has sha256 $actual, this script pins $OMG_SYSML_JSON_SCHEMA_SHA256" >&2
	echo "       the published schema has moved: investigate what changed before re-pinning the digest" >&2
	exit 1
fi

mv "$work" "$target"
trap - EXIT
echo "Installed $target"
echo "Measure the api-json element form against it with:"
echo "  go test -count=1 ./tests/export -run TestAPIJSONNormativeSchema"
