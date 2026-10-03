#!/usr/bin/env bash
# Download the OMG SysML v1 to v2 transformation model (ptc/25-04-10) into
# build/sysml-v1tov2/, for the transformation census in
# tools/cmd/transformation-census.
#
# The model is not vendored: it is OMG's, published under the specification's
# terms, and this project downloads it exactly as it downloads the PSSM test
# suite. The census skips its XMI comparison when the file is absent, so this
# script is optional for building and testing; CI sets
# OPENSYSML_REQUIRE_SYSML_V1TOV2=1 so an absent model fails there.
#
# The URL and checksum live in scripts/sysml-v1tov2-pin.sh. A download whose
# checksum does not match exits non-zero and leaves no file behind. Only the
# model and its pin stamp are ever written to the target; anything else there
# is left alone.
set -euo pipefail

# shellcheck source=scripts/sysml-v1tov2-pin.sh
. "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/sysml-v1tov2-pin.sh"

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
target="${SYSML_V1TOV2_ROOT:-$repo_root/build/sysml-v1tov2}"
model="$target/$SYSML_V1TOV2_FILE"
stamp="$target/.sysml-v1tov2-pin"
pin="$(sysml_v1tov2_pin)"
force=0

case "${1:-}" in
	--force)
		force=1
		;;
	"") ;;
	*)
		echo "error: unknown option: $1 (only --force is supported)" >&2
		exit 1
		;;
esac

# A present model is trusted only with the current stamp and the pinned digest:
# a restored cache can carry an intact stamp over a truncated file.
if [[ "$force" -eq 0 ]] && [[ -f "$model" ]] && [[ -f "$stamp" ]]; then
	if [[ "$(cat "$stamp")" != "$pin" ]]; then
		echo "Stale pin at $target: fetched as $(cat "$stamp"), pin is now $pin; re-downloading."
	elif ! echo "$SYSML_V1TOV2_SHA256  $model" | sha256sum -c --quiet - >/dev/null 2>&1; then
		echo "Corrupt model at $model: sha256 is $(sha256sum "$model" | cut -d' ' -f1), pin is $SYSML_V1TOV2_SHA256; re-downloading."
	else
		echo "Already present at $model ($SYSML_V1TOV2_DOCUMENT, sha256 $SYSML_V1TOV2_SHA256)"
		echo "Remove that directory, or pass --force, to re-download."
		exit 0
	fi
fi

if ! command -v curl >/dev/null 2>&1; then
	echo "error: curl is required to download the SysML v1 to v2 transformation model" >&2
	exit 1
fi

# Staged under the target so a failed fetch leaves no artifact and the renames stay on one filesystem.
mkdir -p "$target"
work="$(mktemp -d "$target/.sysml-v1tov2-fetch.XXXXXX")"
trap 'rm -rf "$work"' EXIT

echo "Fetching $SYSML_V1TOV2_URL ($SYSML_V1TOV2_DOCUMENT) ..."
if ! curl -fsSL --proto '=https' --proto-redir '=https' --retry 3 \
	-o "$work/$SYSML_V1TOV2_FILE" "$SYSML_V1TOV2_URL" 2>"$work/curl.log"; then
	sed 's/^/  /' "$work/curl.log" >&2
	echo "error: could not fetch $SYSML_V1TOV2_URL" >&2
	echo "       omg.org publishes the model at that permanent URL; if the host is unreachable" >&2
	echo "       from this network, fetch it elsewhere, verify its sha256 is $SYSML_V1TOV2_SHA256," >&2
	echo "       and place it at $model with SYSML_V1TOV2_ROOT pointing at its directory." >&2
	exit 1
fi

if ! echo "$SYSML_V1TOV2_SHA256  $work/$SYSML_V1TOV2_FILE" | sha256sum -c --quiet - >/dev/null 2>&1; then
	actual="$(sha256sum "$work/$SYSML_V1TOV2_FILE" | cut -d' ' -f1)"
	echo "error: $SYSML_V1TOV2_FILE from $SYSML_V1TOV2_URL has sha256 $actual," >&2
	echo "       scripts/sysml-v1tov2-pin.sh pins $SYSML_V1TOV2_SHA256" >&2
	echo "       the published file has changed: investigate what changed before re-pinning," >&2
	echo "       or override SYSML_V1TOV2_SHA256 deliberately" >&2
	exit 1
fi

# Only the model and stamp are installed, by rename, stamp last: anything else in the
# target is the caller's, and an interrupted install is re-fetched by the check above.
printf '%s\n' "$pin" >"$work/.sysml-v1tov2-pin"
rm -f "$stamp"
mv -f "$work/$SYSML_V1TOV2_FILE" "$model"
mv -f "$work/.sysml-v1tov2-pin" "$stamp"

echo "Downloaded $SYSML_V1TOV2_FILE ($(wc -c <"$model" | tr -d ' ') bytes) to $target"
echo "Run the census with:"
echo "  go run -C tools ./cmd/transformation-census"
