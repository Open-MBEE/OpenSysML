#!/usr/bin/env bash
# Download the fUML reference implementation's test models, jar and runtime
# dependencies (scripts/fuml-pin.sh) into build/fuml/, checksummed, never vendored;
# see docs/project/fuml-referee.md. Optional: the referee skips when it is absent.
set -euo pipefail

# shellcheck source=scripts/fuml-pin.sh
. "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/fuml-pin.sh"

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
target="${FUML_SUITE_ROOT:-$repo_root/build/fuml}"
libdir="$target/lib"
stamp="$target/.fuml-pin"
pin="$(fuml_pin)"
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

# verify FILE SHA256 succeeds iff the file is present with the pinned digest.
verify() {
	[[ -f "$1" ]] && echo "$2  $1" | sha256sum -c --quiet - >/dev/null 2>&1
}

# present succeeds iff every pinned artefact is installed with its digest.
present() {
	verify "$target/$FUML_TESTS_FILE" "$FUML_TESTS_SHA256" &&
		verify "$target/$FUML_EXCEPTION_TESTS_FILE" "$FUML_EXCEPTION_TESTS_SHA256" &&
		verify "$target/$FUML_LIBRARY_FILE" "$FUML_LIBRARY_SHA256" &&
		verify "$target/$FUML_JAR_FILE" "$FUML_JAR_SHA256" || return 1
	local path sum
	while read -r path sum; do
		verify "$libdir/${path##*/}" "$sum" || return 1
	done <<<"$FUML_DEPS"
	return 0
}

# A present suite is trusted only with the current stamp and the pinned digests:
# a restored cache can carry an intact stamp over a truncated file. Files that do
# verify are kept whatever the stamp says, so a hand-placed suite just gets stamped.
if [[ "$force" -eq 0 ]] && [[ -f "$stamp" ]]; then
	if [[ "$(cat "$stamp")" != "$pin" ]]; then
		echo "Stale pin at $target: fetched as $(cat "$stamp"), pin is now $pin; refreshing what changed."
	elif ! present; then
		echo "Corrupt or incomplete suite at $target; refreshing the missing or altered files."
	else
		echo "Already present at $target ($FUML_RI_TAG, commit $FUML_RI_COMMIT)"
		echo "Remove that directory, or pass --force, to re-download."
		exit 0
	fi
fi

# Staged under the target so a failed fetch leaves no artifact and the renames stay on one filesystem.
mkdir -p "$target"
work="$(mktemp -d "$target/.fuml-fetch.XXXXXX")"
trap 'rm -rf "$work"' EXIT
mkdir -p "$work/lib"

# fetch URL DEST SHA256 LABEL stages the artefact: unless --force, a copy already
# installed at $target/LABEL with the pinned digest is kept (a hand-placed file, or a
# dependency Maven Central would rate-limit re-fetching); otherwise it is downloaded and verified.
fetch() {
	local url="$1" dest="$2" sum="$3" label="$4"
	if [[ "$force" -eq 0 ]] && verify "$target/$label" "$sum"; then
		cp "$target/$label" "$dest"
		return 0
	fi
	if ! command -v curl >/dev/null 2>&1; then
		echo "error: curl is required to download $url" >&2
		echo "       or place a verified copy at $target/$label; see docs/project/fuml-referee.md" >&2
		exit 1
	fi
	echo "Fetching $url ..."
	if ! curl -fsSL --proto '=https' --proto-redir '=https' --retry 3 --retry-all-errors \
		-o "$dest" "$url" 2>"$work/curl.log"; then
		sed 's/^/  /' "$work/curl.log" >&2
		echo "error: could not fetch $url" >&2
		echo "       if the host is unreachable from this network, fetch it elsewhere, verify its" >&2
		echo "       sha256 is $sum, and place it at $target/$label with FUML_SUITE_ROOT" >&2
		echo "       pointing at that directory; run this script again to write the pin stamp." >&2
		exit 1
	fi
	if ! echo "$sum  $dest" | sha256sum -c --quiet - >/dev/null 2>&1; then
		local actual
		actual="$(sha256sum "$dest" | cut -d' ' -f1)"
		echo "error: $label from $url has sha256 $actual," >&2
		echo "       scripts/fuml-pin.sh pins $sum" >&2
		echo "       the published file has changed: investigate what changed before re-pinning," >&2
		echo "       or override the pinned variable deliberately" >&2
		exit 1
	fi
}

fetch "$FUML_TESTS_URL" "$work/$FUML_TESTS_FILE" "$FUML_TESTS_SHA256" "$FUML_TESTS_FILE"
fetch "$FUML_EXCEPTION_TESTS_URL" "$work/$FUML_EXCEPTION_TESTS_FILE" "$FUML_EXCEPTION_TESTS_SHA256" "$FUML_EXCEPTION_TESTS_FILE"
fetch "$FUML_LIBRARY_URL" "$work/$FUML_LIBRARY_FILE" "$FUML_LIBRARY_SHA256" "$FUML_LIBRARY_FILE"
fetch "$FUML_JAR_URL" "$work/$FUML_JAR_FILE" "$FUML_JAR_SHA256" "$FUML_JAR_FILE"
while read -r path sum; do
	fetch "$FUML_MAVEN_REPO/$path" "$work/lib/${path##*/}" "$sum" "lib/${path##*/}"
done <<<"$FUML_DEPS"

# Only the pinned files are installed, by rename, stamp last: anything else in the
# target is the caller's, and an interrupted install is re-fetched by the check above.
printf '%s\n' "$pin" >"$work/.fuml-pin"
rm -f "$stamp"
mkdir -p "$libdir"
for f in "$FUML_TESTS_FILE" "$FUML_EXCEPTION_TESTS_FILE" "$FUML_LIBRARY_FILE" "$FUML_JAR_FILE"; do
	mv -f "$work/$f" "$target/$f"
done
while read -r path _; do
	mv -f "$work/lib/${path##*/}" "$libdir/${path##*/}"
done <<<"$FUML_DEPS"
mv -f "$work/.fuml-pin" "$stamp"

echo "Installed $FUML_TESTS_FILE, $FUML_EXCEPTION_TESTS_FILE, $FUML_LIBRARY_FILE, $FUML_JAR_FILE"
echo "and $(fuml_dep_files | wc -l | tr -d ' ') dependency jars to $target"
echo "Regenerate the oracle record with:"
echo "  make fuml-expected"
echo "Run the referee with:"
echo "  go run -C tools ./cmd/fuml-referee"
