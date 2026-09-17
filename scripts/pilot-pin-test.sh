#!/usr/bin/env bash
# Checks pilot_fetch_subtrees in scripts/pilot-pin.sh against a throwaway release
# repository, so the pin, the stamp and the empty-subtree refusal are the only
# things under test and no network is needed.
set -euo pipefail

pin_script=$(cd "$(dirname "$0")" && pwd)/pilot-pin.sh
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

# A release repository with one tag: a populated subtree, an empty one, and a
# pair the multi-target case fetches together.
release="$work/release"
git -C "$work" init -q release
git -C "$release" config user.email ci@example.com
git -C "$release" config user.name CI
mkdir -p "$release/sysml.library.xmi/Domain" "$release/empty.subtree" "$release/pair/a" "$release/pair/b"

# stub_xmi writes a minimal XMI document at each path given.
stub_xmi() {
	local path
	for path in "$@"; do
		echo '<xmi/>' >"$path"
	done
}
stub_xmi "$release/sysml.library.xmi/Domain/Quantities.sysmlx" "$release/sysml.library.xmi/Kernel.kermlx" \
	"$release/pair/a/A.sysmlx" "$release/pair/b/B.sysmlx"
echo notes >"$release/empty.subtree/README.md"
git -C "$release" add .
git -C "$release" -c commit.gpgsign=false commit -qm release
git -C "$release" tag test-tag
commit=$(git -C "$release" rev-parse HEAD)

failures=0
status=0
output=

# fetch <commit the pin expects> <entries...>: runs pilot_fetch_subtrees the way the
# download scripts do (errexit on, XMI globs), leaving its output and exit status
# in $output and $status.
fetch() {
	local expected_commit=$1
	shift
	status=0
	output=$(
		PILOT_TAG=test-tag PILOT_RELEASE_REPO="file://$release" PILOT_RELEASE_COMMIT="$expected_commit" \
			bash -euo pipefail -c '
				. "$1"; shift
				pilot_from_release
				PILOT_FETCH_GLOBS=("*.sysmlx" "*.kermlx")
				pilot_fetch_subtrees "$@"
			' bash "$pin_script" "$@" 2>&1
	) || status=$?
}

count_xmi() {
	local dir=$1
	find "$dir" -type f \( -name '*.sysmlx' -o -name '*.kermlx' \) 2>/dev/null | wc -l | tr -d ' '
}

pass() {
	echo "ok   $1"
}

fail() {
	echo "FAIL $1" >&2
	printf '     %s\n' "$output" >&2
	failures=$((failures + 1))
}

target="$work/dest/pilot-library-xmi"
pin="test-tag $commit file://$release"

name="a populated subtree is installed and stamped"
fetch "$commit" "sysml.library.xmi:$target"
if [[ $status -eq 0 ]] && [[ $(count_xmi "$target") -eq 2 ]] && [[ $(cat "$target/.pilot-pin") == "$pin" ]]; then
	pass "$name"
else
	fail "$name"
fi

name="a destination at the current pin is left alone"
fetch "$commit" "sysml.library.xmi:$target"
if [[ $status -eq 0 ]] && [[ $output == *"Already present"* ]] && [[ $(count_xmi "$target") -eq 2 ]]; then
	pass "$name"
else
	fail "$name"
fi

name="a stamped destination that holds no file is re-fetched"
find "$target" -type f -not -name .pilot-pin -delete
fetch "$commit" "sysml.library.xmi:$target"
if [[ $status -eq 0 ]] && [[ $output == *"Empty copy"* ]] && [[ $(count_xmi "$target") -eq 2 ]]; then
	pass "$name"
else
	fail "$name"
fi

name="a subtree that holds no file fails and installs nothing"
empty_target="$work/dest/empty"
fetch "$commit" "empty.subtree:$empty_target"
if [[ $status -ne 0 ]] && [[ $output == *"holds no *.sysmlx *.kermlx file"* ]] && [[ ! -e "$empty_target" ]]; then
	pass "$name"
else
	fail "$name"
fi

name="an empty subtree leaves a stale destination as it was"
stale_pin="stale-tag $commit file://$release"
echo "$stale_pin" >"$target/.pilot-pin"
fetch "$commit" "empty.subtree:$target"
if [[ $status -ne 0 ]] && [[ $(count_xmi "$target") -eq 2 ]] && [[ $(cat "$target/.pilot-pin") == "$stale_pin" ]]; then
	pass "$name"
else
	fail "$name"
fi

name="one empty subtree stops every subtree of the fetch from being installed"
pair_a="$work/dest/pair-a"
fetch "$commit" "pair/a:$pair_a" "empty.subtree:$empty_target"
if [[ $status -ne 0 ]] && [[ ! -e "$pair_a" ]] && [[ ! -e "$empty_target" ]]; then
	pass "$name"
else
	fail "$name"
fi

name="several subtrees are installed from one clone"
pair_b="$work/dest/pair-b"
fetch "$commit" "pair/a:$pair_a" "pair/b:$pair_b"
if [[ $status -eq 0 ]] && [[ $(count_xmi "$pair_a") -eq 1 ]] && [[ $(count_xmi "$pair_b") -eq 1 ]]; then
	pass "$name"
else
	fail "$name"
fi

name="a tag that no longer resolves to the pinned commit is refused"
fetch "0000000000000000000000000000000000000000" "sysml.library.xmi:$work/dest/moved"
if [[ $status -ne 0 ]] && [[ $output == *"resolves to $commit"* ]] && [[ ! -e "$work/dest/moved" ]]; then
	pass "$name"
else
	fail "$name"
fi

name="a subtree the release does not carry is refused"
fetch "$commit" "no.such.subtree:$work/dest/missing"
if [[ $status -ne 0 ]] && [[ $output == *"is missing from"* ]] && [[ ! -e "$work/dest/missing" ]]; then
	pass "$name"
else
	fail "$name"
fi

if [[ $failures -gt 0 ]]; then
	echo "$failures pilot-pin case(s) failed" >&2
	exit 1
fi
echo "all pilot-pin cases passed"
