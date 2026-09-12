#!/usr/bin/env bash
# Single source of the OMG SysML v2 Pilot Implementation pin, sourced by every
# script that fetches something from it: the training corpus, the additional OMG
# corpora, the Xpect suites, the grammars, and the reference validators the
# differential harness compares against.
#
# Kept in one file so the release under comparison cannot drift between them.
# The tag names the release; the commit is what every fetch verifies, because a
# tag is a mutable ref and the baselines record content. Change them together.
PILOT_TAG="${PILOT_TAG:-2026-08}"
PILOT_COMMIT="${PILOT_COMMIT:-692170b71867353b8f90341e61556f49a5beb0e5}"
PILOT_REPO="${PILOT_REPO:-https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation.git}"
PILOT_ARTIFACT_VERSION="${PILOT_ARTIFACT_VERSION:-0.62.0}"

# File names pilot_fetch_subtrees counts when reporting a download; a caller may reassign it.
PILOT_FETCH_GLOBS=('*.sysml' '*.kerml')

# pilot_pin is the stamp a fetched destination records.
pilot_pin() {
	printf '%s %s %s' "$PILOT_TAG" "$PILOT_COMMIT" "$PILOT_REPO"
	return 0
}

# pilot_clone sparse-clones the pinned pilot repository into $1 with only the given
# paths checked out, failing unless the tag still resolves to PILOT_COMMIT.
pilot_clone() {
	local dir="$1" head
	shift
	echo "Fetching $* from $PILOT_REPO at $PILOT_TAG ($PILOT_COMMIT) ..."
	if ! git -c advice.detachedHead=false clone --quiet --filter=blob:none --sparse --depth 1 \
		--branch "$PILOT_TAG" "$PILOT_REPO" "$dir"; then
		echo "error: could not clone $PILOT_REPO at $PILOT_TAG, the tag scripts/pilot-pin.sh pins" >&2
		return 1
	fi
	head="$(git -C "$dir" rev-parse 'HEAD^{commit}')"
	if [[ "$head" != "$PILOT_COMMIT" ]]; then
		echo "error: $PILOT_REPO tag $PILOT_TAG resolves to $head, scripts/pilot-pin.sh pins $PILOT_COMMIT" >&2
		echo "       the release tag has moved: investigate what changed before re-pinning PILOT_COMMIT," >&2
		echo "       or override PILOT_COMMIT together with PILOT_TAG deliberately" >&2
		return 1
	fi
	git -C "$dir" sparse-checkout set "$@"
}

# pilot_count_files counts the files under $1 that match PILOT_FETCH_GLOBS.
pilot_count_files() {
	local dir="$1" glob args=()
	for glob in "${PILOT_FETCH_GLOBS[@]}"; do
		args+=(-o -name "$glob")
	done
	find "$dir" -type f \( "${args[@]:1}" \) | wc -l | tr -d ' '
	return 0
}

# pilot_recover_dir puts back the $1.old backup an interrupted pilot_install_dir left behind.
pilot_recover_dir() {
	local dst="$1"
	if [[ ! -e "$dst" ]] && [[ -e "$dst.old" ]]; then
		mv "$dst.old" "$dst"
	fi
	return 0
}

# pilot_install_dir replaces directory $2 with $1, keeping the old copy as
# $2.old until the rename into place succeeds and restoring it if that fails.
pilot_install_dir() {
	local src="$1" dst="$2"
	mkdir -p "$(dirname "$dst")"
	pilot_recover_dir "$dst"
	rm -rf "$dst.new" "$dst.old"
	mv "$src" "$dst.new" || return 1
	if [[ -e "$dst" ]] && ! mv "$dst" "$dst.old"; then
		return 1
	fi
	if ! mv "$dst.new" "$dst"; then
		pilot_recover_dir "$dst"
		return 1
	fi
	rm -rf "$dst.old"
	return 0
}

# pilot_fetch_subtrees copies "<path in the pilot repository>:<destination>" subtrees
# out of one sparse clone; a destination whose .pilot-pin stamp is not the current pin is re-fetched.
pilot_fetch_subtrees() {
	local paths=() targets=() entry source_path target work index pin stamp
	pin="$(pilot_pin)"
	for entry in "$@"; do
		source_path="${entry%%:*}"
		target="${entry#*:}"
		if [[ -d "$target" ]]; then
			stamp="$target/.pilot-pin"
			if [[ -f "$stamp" ]] && [[ "$(cat "$stamp")" == "$pin" ]]; then
				echo "Already present at $target (pin $PILOT_TAG $PILOT_COMMIT)"
				echo "Remove that directory to re-download."
				continue
			fi
			if [[ -f "$stamp" ]]; then
				echo "Stale pin at $target: fetched from $(cat "$stamp"), pin is now $pin; re-downloading."
			else
				echo "No pin recorded at $target: it predates the stamp or was fetched by hand; re-downloading at $PILOT_TAG."
			fi
		fi
		paths+=("$source_path")
		targets+=("$target")
	done
	if [[ "${#paths[@]}" -eq 0 ]]; then
		return 0
	fi

	work="$(mktemp -d)"
	# shellcheck disable=SC2064 # expand $work now; the trap outlives this scope
	trap "rm -rf '$work'" EXIT

	pilot_clone "$work/pilot" "${paths[@]}" || return 1

	for index in "${!paths[@]}"; do
		source_path="${paths[$index]}"
		target="${targets[$index]}"
		if [[ ! -d "$work/pilot/$source_path" ]]; then
			echo "error: $source_path is missing from $PILOT_REPO at $PILOT_TAG" >&2
			return 1
		fi
		printf '%s\n' "$pin" >"$work/pilot/$source_path/.pilot-pin"
		pilot_install_dir "$work/pilot/$source_path" "$target"
		echo "Downloaded $(pilot_count_files "$target") file(s) from $source_path to $target"
	done
}
