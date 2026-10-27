#!/usr/bin/env bash
# Sync the vendored extension libraries from Open-MBEE/OpenSysML-Extensions-Library.
#
# Upstream set: libraries/*.sysml, libraries/*.kerml and engine-contract.json,
# written flat into internal/workspace/libs/stdlib/OpenSysML Libraries/, and
# the project's manifests (.project.json, .meta.json) with its LICENSE, written
# into internal/workspace/libs/extensions-project/. Each directory owns
# README.md only; any other file that is not upstream is an error, so hand
# edits there fail loudly instead of silently surviving a sync.
#
#   scripts/sync-extension-libraries.sh            vendor EXTENSIONS_COMMIT
#   scripts/sync-extension-libraries.sh --ref REF  vendor a branch or tag
#   scripts/sync-extension-libraries.sh --source DIR  vendor a local tree
#   scripts/sync-extension-libraries.sh --check    diff only; exit 1 on drift
set -euo pipefail

# shellcheck source=scripts/extension-libraries-pin.sh
. "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/extension-libraries-pin.sh"

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
vendored="$repo_root/internal/workspace/libs/stdlib/OpenSysML Libraries"
vendored_project="$repo_root/internal/workspace/libs/extensions-project"

check=0
ref=""
source_dir=""
while [[ $# -gt 0 ]]; do
	case "$1" in
		--check)
			check=1
			shift
			;;
		--ref)
			ref="${2:?--ref needs a branch or tag}"
			shift 2
			;;
		--source)
			source_dir="${2:?--source needs a directory}"
			shift 2
			;;
		*)
			echo "error: unknown argument: $1" >&2
			exit 2
			;;
	esac
done
if [[ -n "$ref" && -n "$source_dir" ]]; then
	echo "error: --ref and --source are exclusive" >&2
	exit 2
fi

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
upstream="$work/upstream/libraries"
upstream_project="$work/upstream/project"
mkdir -p "$upstream" "$upstream_project"
# The manifests and the file they record beside the units.
project_files=(.project.json .meta.json LICENSE)

if [[ -n "$source_dir" ]]; then
	if [[ ! -d "$source_dir/libraries" ]]; then
		echo "error: $source_dir has no libraries/ directory" >&2
		exit 1
	fi
	tree="$source_dir"
	resolved="local tree $source_dir"
else
	fetch_ref="$EXTENSIONS_COMMIT"
	[[ -n "$ref" ]] && fetch_ref="$ref"
	git init --quiet "$work/git"
	git -C "$work/git" remote add origin "$EXTENSIONS_REPO"
	echo "Fetching $fetch_ref from $EXTENSIONS_REPO ..."
	if ! git -C "$work/git" fetch --quiet --depth 1 origin "$fetch_ref"; then
		echo "error: could not fetch $fetch_ref from $EXTENSIONS_REPO" >&2
		exit 1
	fi
	resolved="$(git -C "$work/git" rev-parse FETCH_HEAD)"
	if [[ -z "$ref" ]] && [[ "$resolved" != "$EXTENSIONS_COMMIT" ]]; then
		echo "error: fetched $resolved, scripts/extension-libraries-pin.sh pins $EXTENSIONS_COMMIT" >&2
		exit 1
	fi
	echo "Resolved $fetch_ref to $resolved"
	# The fetch has no checkout: stream each upstream file out of the object store.
	while IFS= read -r path; do
		mkdir -p "$work/tree/$(dirname "$path")"
		git -C "$work/git" show "FETCH_HEAD:$path" >"$work/tree/$path"
	done < <(git -C "$work/git" ls-tree -r --name-only FETCH_HEAD -- libraries engine-contract.json "${project_files[@]}")
	tree="$work/tree"
fi
cp "$tree"/libraries/*.sysml "$tree"/libraries/*.kerml "$upstream/" 2>/dev/null || true
if [[ -f "$tree/engine-contract.json" ]]; then
	cp "$tree/engine-contract.json" "$upstream/"
fi
for name in "${project_files[@]}"; do
	if [[ -f "$tree/$name" ]]; then
		cp "$tree/$name" "$upstream_project/"
	fi
done

# An upstream holding no libraries at all is refused: it would empty the
# vendored directory, which is how a sync of a ref that does not carry them
# (such as a branch cut before the import) would silently break the engine.
upstream_libs="$(find "$upstream" -maxdepth 1 -type f \( -name '*.sysml' -o -name '*.kerml' \) | wc -l | tr -d ' ')"
if [[ "$upstream_libs" -eq 0 ]]; then
	echo "error: upstream holds no .sysml or .kerml library file; refusing to sync an empty set" >&2
	exit 1
fi

# Without the contract the sync would delete the vendored manifest as drift.
if [[ ! -f "$upstream/engine-contract.json" ]]; then
	echo "error: upstream holds no engine-contract.json; refusing to sync without the engine contract" >&2
	exit 1
fi

# Without the manifests the sync would delete the vendored project as drift,
# and the vendored units would no longer be provably the published project.
for name in "${project_files[@]}"; do
	if [[ ! -f "$upstream_project/$name" ]]; then
		echo "error: upstream holds no $name; refusing to sync a tree that is not a project interchange" >&2
		exit 1
	fi
done

added=()
removed=()
changed=()
unexpected=()

# compare SRC DST OWNED...: the files under SRC against those under DST,
# recording each path added, changed, removed or unexpected. DST owns the OWNED
# files; any other entry there that is not upstream is unexpected.
compare() {
	local src="$1" dst="$2" file name owned
	shift 2
	for file in "$src"/* "$src"/.[!.]*; do
		[[ -f "$file" ]] || continue
		name="$(basename "$file")"
		if [[ ! -f "$dst/$name" ]]; then
			added+=("$dst/$name")
		elif ! cmp -s "$file" "$dst/$name"; then
			changed+=("$dst/$name")
		fi
	done
	for file in "$dst"/* "$dst"/.[!.]*; do
		[[ -e "$file" ]] || continue
		name="$(basename "$file")"
		# A directory or anything else that is not a file is never upstream's.
		if [[ ! -f "$file" ]]; then
			unexpected+=("$dst/$name")
			continue
		fi
		[[ -f "$src/$name" ]] && continue
		for owned in "$@"; do
			[[ "$name" == "$owned" ]] && continue 2
		done
		case "$name" in
			*.sysml | *.kerml | engine-contract.json | .project.json | .meta.json | LICENSE) removed+=("$dst/$name") ;;
			*) unexpected+=("$dst/$name") ;;
		esac
	done
}
compare "$upstream" "$vendored" README.md
compare "$upstream_project" "$vendored_project" README.md

if [[ "${#unexpected[@]}" -gt 0 ]]; then
	printf 'error: unexpected file(s): %s\n' "${unexpected[*]#"$repo_root"/}" >&2
	echo "       those directories are written by this script; remove the files or move them upstream" >&2
	exit 1
fi

drift=$(( ${#added[@]} + ${#removed[@]} + ${#changed[@]} ))
if [[ "$drift" -eq 0 ]]; then
	echo "Extension libraries in sync with $resolved"
	exit 0
fi

for name in "${added[@]}"; do echo "  added    ${name#"$repo_root"/}"; done
for name in "${changed[@]}"; do echo "  changed  ${name#"$repo_root"/}"; done
for name in "${removed[@]}"; do echo "  removed  ${name#"$repo_root"/}"; done

if [[ "$check" -eq 1 ]]; then
	echo "error: the vendored extension libraries differ from $resolved" >&2
	echo "       edits go upstream to Open-MBEE/OpenSysML-Extensions-Library;" >&2
	echo "       bump scripts/extension-libraries-pin.sh and run scripts/sync-extension-libraries.sh" >&2
	exit 1
fi

# source_of DST/name: the staged upstream copy of a vendored path.
source_of() {
	case "$1" in
		"$vendored_project"/*) echo "$upstream_project/$(basename "$1")" ;;
		*) echo "$upstream/$(basename "$1")" ;;
	esac
}
mkdir -p "$vendored_project"
for name in "${removed[@]}"; do rm -f "$name"; done
for name in "${added[@]}" "${changed[@]}"; do cp "$(source_of "$name")" "$name"; done
echo "Synced extension libraries to $resolved"
