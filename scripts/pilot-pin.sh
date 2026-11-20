#!/usr/bin/env bash
# Single source of the OMG SysML v2 Pilot Implementation pin, sourced by every
# script that fetches something from it: the training corpus, the additional OMG
# corpora, the Xpect suites, the grammars, and the reference validators the
# differential harness compares against.
#
# Kept in one file so the release under comparison cannot drift between them.
# The tag names the release; the commit is what every fetch verifies, because a
# tag is a mutable ref and the baselines record content. Change them together.
PILOT_TAG="${PILOT_TAG:-2026-09.1}"
PILOT_COMMIT="${PILOT_COMMIT:-d81b2ad5016f2e2d3d4364b3d06d01d8bb12a896}"
PILOT_REPO="${PILOT_REPO:-https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation.git}"
PILOT_ARTIFACT_VERSION="${PILOT_ARTIFACT_VERSION:-0.63.1}"
# The Java feature release a baseline holding the pilot's export is recorded under, the one
# the workflows install: the pilot's output is not the same under every JVM.
PILOT_JAVA="${PILOT_JAVA:-21}"

# SHA-256 of the release's kernel jar and (as pilot_tree_sha256) of the library beside it: a
# release asset is mutable. They move with PILOT_ARTIFACT_VERSION; pilot_kernel_digests prints them.
PILOT_KERNEL_JAR_SHA256="${PILOT_KERNEL_JAR_SHA256:-67c914092f4520ba98548b285c5a9a8bc08f92a0aeec7b763e12a574e91f9ff7}"
PILOT_KERNEL_LIBRARY_SHA256="${PILOT_KERNEL_LIBRARY_SHA256:-a59332f5fcbfccf3634cc786cdd271876f2cd7877536ea042d1fe6334eb26b4e}"

# The pilot publishes each release's standard library, in notation and as XMI,
# in a second repository under the same tag. The bundled library under
# internal/workspace/libs/stdlib is its sysml.library tree; sysml.library.xmi carries
# the normative element ids the identity gate compares against. Pinned by commit
# like the pilot itself; the two move together at a release.
PILOT_RELEASE_REPO="${PILOT_RELEASE_REPO:-https://github.com/Systems-Modeling/SysML-v2-Release.git}"
PILOT_RELEASE_COMMIT="${PILOT_RELEASE_COMMIT:-9546486f49f704d1df092e3f252f4fcfbce534dd}"

# The repository and commit pilot_clone and pilot_fetch_subtrees read from: the
# pilot by default, the release repository after pilot_from_release.
PILOT_SOURCE_REPO="$PILOT_REPO"
PILOT_SOURCE_COMMIT="$PILOT_COMMIT"

# pilot_from_release points the fetch functions at the release repository.
pilot_from_release() {
	PILOT_SOURCE_REPO="$PILOT_RELEASE_REPO"
	PILOT_SOURCE_COMMIT="$PILOT_RELEASE_COMMIT"
	return 0
}

# File names pilot_fetch_subtrees counts: a subtree with none is refused. A caller may reassign it.
PILOT_FETCH_GLOBS=('*.sysml' '*.kerml')

# pilot_pin is the stamp a fetched destination records.
pilot_pin() {
	printf '%s %s %s' "$PILOT_TAG" "$PILOT_SOURCE_COMMIT" "$PILOT_SOURCE_REPO"
	return 0
}

# pilot_clone sparse-clones the pinned source repository into $1 with only the given
# paths (files or directories) checked out, failing unless the tag still resolves to the pinned commit.
pilot_clone() {
	local dir="$1" head path patterns=()
	shift
	echo "Fetching $* from $PILOT_SOURCE_REPO at $PILOT_TAG ($PILOT_SOURCE_COMMIT) ..."
	if ! git -c advice.detachedHead=false clone --quiet --filter=blob:none --sparse --depth 1 \
		--branch "$PILOT_TAG" "$PILOT_SOURCE_REPO" "$dir"; then
		echo "error: could not clone $PILOT_SOURCE_REPO at $PILOT_TAG, the tag scripts/pilot-pin.sh pins" >&2
		return 1
	fi
	head="$(git -C "$dir" rev-parse 'HEAD^{commit}')"
	if [[ "$head" != "$PILOT_SOURCE_COMMIT" ]]; then
		echo "error: $PILOT_SOURCE_REPO tag $PILOT_TAG resolves to $head, scripts/pilot-pin.sh pins $PILOT_SOURCE_COMMIT" >&2
		echo "       the release tag has moved: investigate what changed before re-pinning the commit," >&2
		echo "       or override it together with PILOT_TAG deliberately" >&2
		return 1
	fi
	# Patterns anchored at the root, not cone mode: a cone names directories only, and
	# git 2.56 refuses a file path there (download-pilot-grammars.sh names .xtext files).
	for path in "$@"; do
		patterns+=("/${path#/}")
	done
	git -C "$dir" -c core.sparseCheckoutCone=false sparse-checkout set "${patterns[@]}"
}

# pilot_sha256 prints the SHA-256 of file $1, or of standard input when none is given.
pilot_sha256() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$@" | cut -d' ' -f1
	else
		shasum -a 256 "$@" | cut -d' ' -f1
	fi
}

# pilot_tree_sha256 prints one digest of directory $1 over a line per entry in byte order: a
# file's digest and path, and the path alone for a link or other non-file, which a reader follows.
pilot_tree_sha256() {
	local dir="$1" entry
	(
		cd "$dir" || exit 1
		find . -mindepth 1 ! -type d | LC_ALL=C sort | while IFS= read -r entry; do
			if [[ -f "$entry" ]] && [[ ! -L "$entry" ]]; then
				printf '%s  %s\n' "$(pilot_sha256 "$entry")" "$entry"
			else
				printf 'not a regular file  %s\n' "$entry"
			fi
		done
	) | pilot_sha256
}

# pilot_kernel_digests prints the two digests of the kernel jar $1 and library directory $2.
pilot_kernel_digests() {
	printf 'PILOT_KERNEL_JAR_SHA256=%s\nPILOT_KERNEL_LIBRARY_SHA256=%s\n' "$(pilot_sha256 "$1")" "$(pilot_tree_sha256 "$2")"
}

# pilot_verify_kernel fails unless the kernel jar $1 and the library directory $2 are the
# pinned ones; every script that runs or compiles against the jar calls it first.
pilot_verify_kernel() {
	local jar="$1" library="$2" got
	if [[ ! -f "$jar" ]] || [[ ! -d "$library" ]]; then
		echo "error: the pilot kernel jar $jar or its library $library is missing" >&2
		return 1
	fi
	got="$(pilot_sha256 "$jar")"
	if [[ "$got" != "$PILOT_KERNEL_JAR_SHA256" ]]; then
		echo "error: $jar has SHA-256 $got, scripts/pilot-pin.sh pins $PILOT_KERNEL_JAR_SHA256" >&2
		echo "       the release asset or the cached build has changed: investigate before re-pinning" >&2
		return 1
	fi
	got="$(pilot_tree_sha256 "$library")"
	if [[ "$got" != "$PILOT_KERNEL_LIBRARY_SHA256" ]]; then
		echo "error: $library has tree SHA-256 $got, scripts/pilot-pin.sh pins $PILOT_KERNEL_LIBRARY_SHA256" >&2
		echo "       the release asset or the cached build has changed: investigate before re-pinning" >&2
		return 1
	fi
	return 0
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
	if ! mv "$src" "$dst.new"; then
		rm -rf "$dst.new"
		return 1
	fi
	if [[ -e "$dst" ]] && ! mv "$dst" "$dst.old"; then
		rm -rf "$dst.new"
		return 1
	fi
	if ! mv "$dst.new" "$dst"; then
		pilot_recover_dir "$dst"
		rm -rf "$dst.new"
		return 1
	fi
	rm -rf "$dst.old"
	return 0
}

# pilot_fetch_subtrees copies "<path in the pilot repository>:<destination>" subtrees
# out of one sparse clone; a destination whose .pilot-pin stamp is not the current pin, or
# that holds no PILOT_FETCH_GLOBS file, is re-fetched. A subtree that holds none fails the
# fetch and installs nothing, so a destination is never stamped complete while empty.
pilot_fetch_subtrees() {
	local paths=() targets=() counts=() entry source_path target work index pin stamp
	pin="$(pilot_pin)"
	for entry in "$@"; do
		source_path="${entry%%:*}"
		target="${entry#*:}"
		if [[ -d "$target" ]]; then
			stamp="$target/.pilot-pin"
			if [[ -f "$stamp" ]] && [[ "$(cat "$stamp")" == "$pin" ]]; then
				if [[ "$(pilot_count_files "$target")" -gt 0 ]]; then
					echo "Already present at $target (pin $PILOT_TAG $PILOT_SOURCE_COMMIT)"
					echo "Remove that directory to re-download."
					continue
				fi
				echo "Empty copy at $target: stamped $PILOT_TAG but holding no ${PILOT_FETCH_GLOBS[*]} file; re-downloading."
			elif [[ -f "$stamp" ]]; then
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

	# Every subtree is checked before any is installed, so a bad one leaves them all as they were.
	for index in "${!paths[@]}"; do
		source_path="${paths[$index]}"
		if [[ ! -d "$work/pilot/$source_path" ]]; then
			echo "error: $source_path is missing from $PILOT_SOURCE_REPO at $PILOT_TAG" >&2
			return 1
		fi
		counts[index]="$(pilot_count_files "$work/pilot/$source_path")"
		if [[ "${counts[$index]}" -eq 0 ]]; then
			echo "error: $source_path at $PILOT_SOURCE_REPO $PILOT_TAG holds no ${PILOT_FETCH_GLOBS[*]} file; ${targets[$index]} is left as it was" >&2
			return 1
		fi
	done

	for index in "${!paths[@]}"; do
		source_path="${paths[$index]}"
		target="${targets[$index]}"
		printf '%s\n' "$pin" >"$work/pilot/$source_path/.pilot-pin"
		pilot_install_dir "$work/pilot/$source_path" "$target" || return 1
		echo "Downloaded ${counts[$index]} file(s) from $source_path to $target"
	done
}
