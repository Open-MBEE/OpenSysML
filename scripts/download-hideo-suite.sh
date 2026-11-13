#!/usr/bin/env bash

# Check out the cross-MDK suite hideo67/sysmlv2-tests at its pinned commit into build/hideo-suite/
# and link this repository as its OpenSysML sibling; see docs/project/hideo-suite.md.
set -euo pipefail

# shellcheck source=scripts/hideo-suite-pin.sh
. "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/hideo-suite-pin.sh"

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
target="${HIDEO_SUITE_ROOT:-$repo_root/build/hideo-suite}"
stamp="$target/.hideo-suite-pin"
link="$target/OpenSysML"
pin="$(hideo_suite_pin)"
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

# count_cases counts the generated case files of the checkout at $1.
count_cases() {
	find "$1/test-suite/data/cases" -type f -name '*.json' 2>/dev/null | wc -l | tr -d ' '
	return 0
}

# link_mdk points the checkout's OpenSysML sibling at this repository.
link_mdk() {
	if [[ -e "$link" ]] && [[ ! -L "$link" ]]; then
		echo "error: $link exists and is not a symbolic link; the suite must read this repository there" >&2
		return 1
	fi
	ln -sfn "$repo_root" "$link"
	return 0
}

if ! command -v git >/dev/null 2>&1; then
	echo "error: git is required to fetch the suite" >&2
	exit 1
fi

# A present checkout is trusted only with the current stamp, the pinned commit
# checked out, no tracked file changed and at least one case file.
if [[ "$force" -eq 0 ]] && [[ -d "$target/.git" ]] && [[ -f "$stamp" ]]; then
	head="$(git -C "$target" rev-parse 'HEAD^{commit}' 2>/dev/null || true)"
	if [[ "$(cat "$stamp")" != "$pin" ]]; then
		echo "Stale pin at $target: fetched as $(cat "$stamp"), pin is now $pin; re-downloading."
	elif [[ "$head" != "$HIDEO_SUITE_COMMIT" ]]; then
		echo "Checkout at $target is at ${head:-no commit}, pin is $HIDEO_SUITE_COMMIT; re-downloading."
	elif [[ -n "$(git -C "$target" status --porcelain --untracked-files=no)" ]]; then
		echo "Checkout at $target has modified tracked files; re-downloading."
	elif [[ "$(count_cases "$target")" -eq 0 ]]; then
		echo "Empty checkout at $target: stamped but holding no case file; re-downloading."
	else
		link_mdk
		echo "Already present at $target (commit $HIDEO_SUITE_COMMIT, $(count_cases "$target") case files)"
		echo "Remove that directory, or pass --force, to re-download."
		exit 0
	fi
fi

# Staged beside the target so a failed fetch installs nothing and the rename stays on one filesystem.
mkdir -p "$(dirname "$target")"
work="$(mktemp -d "$(dirname "$target")/.hideo-suite-fetch.XXXXXX")"
trap 'rm -rf "$work"' EXIT

echo "Fetching $HIDEO_SUITE_REPO at $HIDEO_SUITE_COMMIT ..."
git -c init.defaultBranch=pin init --quiet "$work/suite"
if ! git -C "$work/suite" fetch --quiet --depth 1 "$HIDEO_SUITE_REPO" "$HIDEO_SUITE_COMMIT"; then
	echo "error: could not fetch $HIDEO_SUITE_COMMIT from $HIDEO_SUITE_REPO, the commit scripts/hideo-suite-pin.sh pins" >&2
	exit 1
fi
git -C "$work/suite" -c advice.detachedHead=false checkout --quiet FETCH_HEAD

head="$(git -C "$work/suite" rev-parse 'HEAD^{commit}')"
if [[ "$head" != "$HIDEO_SUITE_COMMIT" ]]; then
	echo "error: fetched $head from $HIDEO_SUITE_REPO, scripts/hideo-suite-pin.sh pins $HIDEO_SUITE_COMMIT" >&2
	exit 1
fi
cases="$(count_cases "$work/suite")"
if [[ "$cases" -eq 0 ]] || [[ ! -x "$work/suite/test-suite/run-all.sh" ]]; then
	echo "error: $HIDEO_SUITE_REPO at $HIDEO_SUITE_COMMIT holds $cases case file(s) under test-suite/data/cases;" >&2
	echo "       at least one, and an executable test-suite/run-all.sh, are required. $target is left as it was" >&2
	exit 1
fi

# The stamp is written last, inside the staged checkout, so an interrupted install is re-fetched.
printf '%s\n' "$pin" >"$work/suite/.hideo-suite-pin"
# The old checkout is kept as $target.old until the new one is in place, and put back if the rename fails.
rm -rf "$target.old"
if [[ -e "$target" ]]; then
	mv "$target" "$target.old"
fi
if ! mv "$work/suite" "$target"; then
	if [[ -e "$target.old" ]]; then
		mv "$target.old" "$target"
	fi
	echo "error: could not install the checkout at $target" >&2
	exit 1
fi
rm -rf "$target.old"
link_mdk

echo "Checked out $cases case file(s) of $HIDEO_SUITE_REPO at $HIDEO_SUITE_COMMIT to $target"
echo "Run the ratchet with:"
echo "  go run -C tools ./cmd/hideo-suite -check"
