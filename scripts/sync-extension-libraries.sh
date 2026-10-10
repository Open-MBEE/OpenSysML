#!/usr/bin/env bash
# Sync the vendored extension libraries from Open-MBEE/OpenSysML-Extensions-Library.
#
# Upstream set: libraries/*.sysml, libraries/*.kerml and engine-contract.json,
# written flat into internal/workspace/libs/stdlib/OpenSysML Libraries/. That
# directory owns README.md only; any other file that is not upstream is an
# error, so hand edits there fail loudly instead of silently surviving a sync.
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
upstream="$work/upstream"
mkdir -p "$upstream"

if [[ -n "$source_dir" ]]; then
	if [[ ! -d "$source_dir/libraries" ]]; then
		echo "error: $source_dir has no libraries/ directory" >&2
		exit 1
	fi
	cp "$source_dir"/libraries/*.sysml "$source_dir"/libraries/*.kerml "$upstream/" 2>/dev/null || true
	if [[ -f "$source_dir/engine-contract.json" ]]; then
		cp "$source_dir/engine-contract.json" "$upstream/"
	fi
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
	done < <(git -C "$work/git" ls-tree -r --name-only FETCH_HEAD -- libraries engine-contract.json)
	cp "$work/tree"/libraries/*.sysml "$work/tree"/libraries/*.kerml "$upstream/" 2>/dev/null || true
	if [[ -f "$work/tree/engine-contract.json" ]]; then
		cp "$work/tree/engine-contract.json" "$upstream/"
	fi
fi

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

added=()
removed=()
changed=()
unexpected=()

for file in "$upstream"/*; do
	name="$(basename "$file")"
	if [[ ! -f "$vendored/$name" ]]; then
		added+=("$name")
	elif ! cmp -s "$file" "$vendored/$name"; then
		changed+=("$name")
	fi
done
for file in "$vendored"/*.sysml "$vendored"/*.kerml "$vendored/engine-contract.json"; do
	[[ -e "$file" ]] || continue
	name="$(basename "$file")"
	[[ -f "$upstream/$name" ]] || removed+=("$name")
done
# Anything else in the vendored directory that is neither upstream nor the
# OpenSysML-owned README.md is unexpected.
for file in "$vendored"/*; do
	name="$(basename "$file")"
	[[ "$name" == "README.md" ]] && continue
	[[ -f "$upstream/$name" ]] && continue
	case "$name" in
		*.sysml | *.kerml | engine-contract.json) ;; # already in removed[]
		*) unexpected+=("$name") ;;
	esac
done

if [[ "${#unexpected[@]}" -gt 0 ]]; then
	printf 'error: unexpected file(s) in %s: %s\n' "$vendored" "${unexpected[*]}" >&2
	echo "       that directory is written by this script; remove them or move them upstream" >&2
	exit 1
fi

drift=$(( ${#added[@]} + ${#removed[@]} + ${#changed[@]} ))
if [[ "$drift" -eq 0 ]]; then
	echo "Extension libraries in sync with $resolved"
	exit 0
fi

for name in "${added[@]}"; do echo "  added    $name"; done
for name in "${changed[@]}"; do echo "  changed  $name"; done
for name in "${removed[@]}"; do echo "  removed  $name"; done

if [[ "$check" -eq 1 ]]; then
	echo "error: the vendored extension libraries differ from $resolved" >&2
	echo "       edits go upstream to Open-MBEE/OpenSysML-Extensions-Library;" >&2
	echo "       bump scripts/extension-libraries-pin.sh and run scripts/sync-extension-libraries.sh" >&2
	exit 1
fi

for name in "${removed[@]}"; do rm -f "$vendored/$name"; done
for name in "${added[@]}" "${changed[@]}"; do cp "$upstream/$name" "$vendored/$name"; done
echo "Synced extension libraries to $resolved"
