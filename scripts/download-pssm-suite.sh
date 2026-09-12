#!/usr/bin/env bash
# Download the OMG PSSM state-machine test suite (ptc/18-11-06) into build/pssm/,
# for the advisory referee in cmd/pssm-referee.
#
# The suite is not vendored: it is OMG's, published under the specification's
# terms, and this project downloads it exactly as it downloads the pilot corpora
# (see docs/project/pssm-referee.md, "The licence reading"). The referee skips
# when the directory is absent, so this script is optional for building and
# testing; CI sets OPENSYSML_REQUIRE_PSSM_SUITE=1 so an absent suite fails there.
#
# The URL and checksum live in scripts/pssm-pin.sh. A download whose checksum
# does not match exits non-zero and leaves nothing behind in build/pssm/.
set -euo pipefail

# shellcheck source=scripts/pssm-pin.sh
. "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/pssm-pin.sh"

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
target="${PSSM_SUITE_ROOT:-$repo_root/build/pssm}"
suite="$target/$PSSM_SUITE_FILE"
stamp="$target/.pssm-pin"
pin="$(pssm_pin)"
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

# A present suite is trusted only with the current stamp and the pinned digest:
# a restored cache can carry an intact stamp over a truncated file.
if [[ "$force" -eq 0 ]] && [[ -f "$suite" ]] && [[ -f "$stamp" ]]; then
	if [[ "$(cat "$stamp")" != "$pin" ]]; then
		echo "Stale pin at $target: fetched as $(cat "$stamp"), pin is now $pin; re-downloading."
	elif ! echo "$PSSM_SUITE_SHA256  $suite" | sha256sum -c --quiet - >/dev/null 2>&1; then
		echo "Corrupt suite at $suite: sha256 is $(sha256sum "$suite" | cut -d' ' -f1), pin is $PSSM_SUITE_SHA256; re-downloading."
	else
		echo "Already present at $suite ($PSSM_DOCUMENT, sha256 $PSSM_SUITE_SHA256)"
		echo "Remove that directory, or pass --force, to re-download."
		exit 0
	fi
fi

if ! command -v curl >/dev/null 2>&1; then
	echo "error: curl is required to download the PSSM test suite" >&2
	exit 1
fi

# Staged outside the target so a failed download or checksum leaves no artifact behind.
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

echo "Fetching $PSSM_SUITE_URL ($PSSM_DOCUMENT) ..."
if ! curl -fsSL --proto '=https' --proto-redir '=https' --retry 3 \
	-o "$work/$PSSM_SUITE_FILE" "$PSSM_SUITE_URL" 2>"$work/curl.log"; then
	sed 's/^/  /' "$work/curl.log" >&2
	echo "error: could not fetch $PSSM_SUITE_URL" >&2
	echo "       omg.org publishes the suite at that permanent URL; if the host is unreachable" >&2
	echo "       from this network, fetch it elsewhere, verify its sha256 is $PSSM_SUITE_SHA256," >&2
	echo "       and place it at $suite with PSSM_SUITE_ROOT pointing at its directory." >&2
	exit 1
fi

if ! echo "$PSSM_SUITE_SHA256  $work/$PSSM_SUITE_FILE" | sha256sum -c --quiet - >/dev/null 2>&1; then
	actual="$(sha256sum "$work/$PSSM_SUITE_FILE" | cut -d' ' -f1)"
	echo "error: $PSSM_SUITE_FILE from $PSSM_SUITE_URL has sha256 $actual," >&2
	echo "       scripts/pssm-pin.sh pins $PSSM_SUITE_SHA256" >&2
	echo "       the published file has changed: investigate what changed before re-pinning," >&2
	echo "       or override PSSM_SUITE_SHA256 deliberately" >&2
	exit 1
fi

rm -f "$work/curl.log"
printf '%s\n' "$pin" >"$work/.pssm-pin"
mkdir -p "$(dirname "$target")"
rm -rf "$target.new"
mv "$work" "$target.new"
trap - EXIT
rm -rf "$target"
mv "$target.new" "$target"

echo "Downloaded $PSSM_SUITE_FILE ($(wc -c <"$suite" | tr -d ' ') bytes) to $target"
echo "Run the referee with:"
echo "  go run ./cmd/pssm-referee"
