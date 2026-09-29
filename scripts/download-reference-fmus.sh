#!/usr/bin/env bash
# Download the Modelica Reference-FMUs (https://github.com/modelica/Reference-FMUs)
# into examples/reference-fmus/, for the FMI gate in tests/fmi and the reference
# runner's real test in client/python.
#
# The FMUs are not vendored: they are the Modelica Association's, published under
# the BSD-3-Clause licence the zip's LICENSE.txt carries, and this project
# downloads them exactly as it downloads the pilot corpora. The gate skips when
# the directory is absent, so this script is optional for building and testing;
# CI sets OPENSYSML_REQUIRE_REFERENCE_FMUS=1 so an absent directory fails there.
#
# The URL and checksum live in scripts/reference-fmus-pin.sh. A download whose
# checksum does not match exits non-zero and leaves no file behind. Only the
# extracted FMUs and the pin stamp are ever written to the target; anything else
# there is left alone.
set -euo pipefail

# shellcheck source=scripts/reference-fmus-pin.sh
. "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/reference-fmus-pin.sh"

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
target="${REFERENCE_FMUS_ROOT:-$repo_root/examples/reference-fmus}"
archive="$target/$REFERENCE_FMUS_FILE"
stamp="$target/.reference-fmus-pin"
pin="$(reference_fmus_pin)"
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

# A present download is trusted only when its stamp names the current pin and
# the extracted tree holds the gate's FMUs: a restored cache can carry an intact
# stamp over a truncated install.
if [[ "$force" -eq 0 ]] && [[ -d "$target/2.0" ]] && [[ -f "$target/2.0/BouncingBall.fmu" ]] && [[ -f "$target/3.0/BouncingBall.fmu" ]] && [[ -f "$stamp" ]] && [[ "$(cat "$stamp")" == "$pin" ]]; then
	echo "Already present at $target (Reference-FMUs $REFERENCE_FMUS_TAG)"
	echo "Remove that directory, or pass --force, to re-download."
	exit 0
fi

if ! command -v curl >/dev/null 2>&1; then
	echo "error: curl is required to download the Reference-FMUs" >&2
	exit 1
fi
if ! command -v unzip >/dev/null 2>&1; then
	echo "error: unzip is required to extract the Reference-FMUs" >&2
	exit 1
fi

# Staged under the target so a failed fetch or extract leaves no artifact and
# the renames stay on one filesystem.
mkdir -p "$target"
work="$(mktemp -d "$target/.reference-fmus-fetch.XXXXXX")"
trap 'rm -rf "$work"' EXIT

echo "Fetching $REFERENCE_FMUS_URL ($REFERENCE_FMUS_TAG) ..."
if ! curl -fsSL --proto '=https' --proto-redir '=https' --retry 3 \
	-o "$work/$REFERENCE_FMUS_FILE" "$REFERENCE_FMUS_URL" 2>"$work/curl.log"; then
	sed 's/^/  /' "$work/curl.log" >&2
	echo "error: could not fetch $REFERENCE_FMUS_URL" >&2
	echo "       GitHub publishes the asset at that permanent URL; if the host is unreachable" >&2
	echo "       from this network, fetch it elsewhere, verify its sha256 is $REFERENCE_FMUS_SHA256," >&2
	echo "       and unpack it under $target with REFERENCE_FMUS_ROOT pointing at its directory." >&2
	exit 1
fi

if ! echo "$REFERENCE_FMUS_SHA256  $work/$REFERENCE_FMUS_FILE" | sha256sum -c --quiet - >/dev/null 2>&1; then
	actual="$(sha256sum "$work/$REFERENCE_FMUS_FILE" | cut -d' ' -f1)"
	echo "error: $REFERENCE_FMUS_FILE from $REFERENCE_FMUS_URL has sha256 $actual," >&2
	echo "       scripts/reference-fmus-pin.sh pins $REFERENCE_FMUS_SHA256" >&2
	echo "       the published file has changed: investigate what changed before re-pinning," >&2
	echo "       or override REFERENCE_FMUS_SHA256 deliberately" >&2
	exit 1
fi

unzip -q "$work/$REFERENCE_FMUS_FILE" -d "$work/extract"

# The extracted tree must hold what the gate reads; an empty or rearranged
# release is a failure here, not a green skip there.
for fmu in 2.0/BouncingBall.fmu 3.0/BouncingBall.fmu; do
	if [[ ! -f "$work/extract/$fmu" ]]; then
		echo "error: $REFERENCE_FMUS_FILE does not contain $fmu; the pin names a different release" >&2
		exit 1
	fi
done

# Only the extracted files and the stamp are installed, stamp last: anything else
# in the target is the caller's, and an interrupted install is re-fetched by the
# check above. The stale tree is replaced whole so a re-pinned layout cannot mix.
rm -rf "$target/2.0" "$target/3.0" "$target/README.md" "$target/LICENSE.txt" "$archive"
mv -f "$work/extract/2.0" "$work/extract/3.0" "$target/"
mv -f "$work/extract/README.md" "$work/extract/LICENSE.txt" "$target/"
printf '%s\n' "$pin" >"$work/.reference-fmus-pin"
mv -f "$work/.reference-fmus-pin" "$stamp"

echo "Downloaded Reference-FMUs $REFERENCE_FMUS_TAG to $target"
echo "Run the gate with:"
echo "  go test -count=1 -v ./tests/fmi"
