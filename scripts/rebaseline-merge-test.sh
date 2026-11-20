#!/usr/bin/env bash
# Checks scripts/rebaseline-merge.sh against throwaway repositories: which conflicts it settles
# with develop's side, which it refuses, and the commits it reports to the movement check.
set -euo pipefail

merge_script=$(cd "$(dirname "$0")" && pwd)/rebaseline-merge.sh
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
failures=0
status=0
output=

# repository <name> <path>: a clone whose branch and origin/develop each rewrite <path>, and so conflict on it.
repository() {
	local repo="$work/$1" path=$2
	git init -q -b develop "$repo"
	git -C "$repo" config user.email ci@example.com
	git -C "$repo" config user.name CI
	git -C "$repo" config commit.gpgsign false
	mkdir -p "$repo/$(dirname "$path")"
	echo base >"$repo/$path"
	echo base >"$repo/source.go"
	git -C "$repo" add .
	git -C "$repo" commit -qm base
	git -C "$repo" checkout -q -b feat/branch
	echo ours >"$repo/$path"
	git -C "$repo" commit -qam ours
	git -C "$repo" checkout -q develop
	echo theirs >"$repo/$path"
	echo theirs >"$repo/source.go"
	git -C "$repo" commit -qam theirs
	git -C "$repo" update-ref refs/remotes/origin/develop develop
	git -C "$repo" checkout -q feat/branch
}

# merge <name> [head]: runs the script in the clone, leaving its output and exit status in $output and $status.
merge() {
	local repo="$work/$1"
	local head=${2:-$(git -C "$repo" rev-parse HEAD)}
	status=0
	output=$(cd "$repo" && GITHUB_OUTPUT="$repo.outputs" BRANCH=feat/branch \
		GENERATED_PATHS=$'tests/corpus/testdata/spec_examples_expected.txt\ntests/export/testdata/convert\n' \
		"$merge_script" "$head" 2>&1) || status=$?
}

check() {
	local name=$1
	shift
	if "$@"; then
		echo "ok: $name"
	else
		echo "FAIL: $name (exit $status)"
		printf '%s\n' "$output" | sed 's/^/  /'
		failures=$((failures + 1))
	fi
}

# merged_as <name> <path> <content>: the merge succeeded, is committed, holds <content> at <path> and
# develop's source, and reported the four commits.
merged_as() {
	local repo="$work/$1" path=$2 content=$3
	[ "$status" = 0 ] && [ -z "$(git -C "$repo" status --porcelain)" ] &&
		[ "$(cat "$repo/$path")" = "$content" ] && [ "$(cat "$repo/source.go")" = theirs ] &&
		grep -qx "merged=$(git -C "$repo" rev-parse HEAD)" "$repo.outputs" &&
		grep -qx "ours=$(git -C "$repo" rev-parse HEAD^1)" "$repo.outputs" &&
		grep -qx "theirs=$(git -C "$repo" rev-parse origin/develop)" "$repo.outputs" &&
		grep -qx "base=$(git -C "$repo" merge-base HEAD^1 origin/develop)" "$repo.outputs"
}

# refused <name> <message>: the script failed with <message> and committed nothing.
refused() {
	local repo="$work/$1" message=$2
	[ "$status" != 0 ] && [[ "$output" == *"$message"* ]] && ! grep -q '^merged=' "$repo.outputs" 2>/dev/null
}

repository expectation tests/corpus/testdata/spec_examples_expected.txt
merge expectation
check "a conflict on a generated file takes develop's side" merged_as expectation tests/corpus/testdata/spec_examples_expected.txt theirs

repository golden tests/export/testdata/convert/a.golden.ttl
merge golden
check "a conflict on a golden under a generated directory takes develop's side" merged_as golden tests/export/testdata/convert/a.golden.ttl theirs

repository fixture tests/export/testdata/convert/a.sysml
merge fixture
check "a conflict on a source fixture under a generated directory is refused" refused fixture "conflicts outside the generated paths"

repository training tests/corpus/testdata/training_examples_expected.txt
merge training
check "a conflict on the training expectation is refused" refused training "tests/corpus/testdata/training_examples_expected.txt"

repository source internal/a.go
merge source
check "a conflict outside the generated paths is refused" refused source "internal/a.go"

repository moved tests/corpus/testdata/spec_examples_expected.txt
merge moved "$(git -C "$work/moved" rev-parse HEAD^)"
check "a head that is not the one named is refused" refused moved "the branch moved"

repository current tests/corpus/testdata/spec_examples_expected.txt
merge current
merge current
already_merged() {
	[ "$status" = 0 ] && [[ "$output" == *"develop is already merged"* ]] &&
		[ "$(tail -n 1 "$work/current.outputs")" = "merged=$(git -C "$work/current" rev-parse HEAD)" ]
}
check "a branch that holds develop is left as it is" already_merged

if [ "$failures" -gt 0 ]; then
	echo "$failures check(s) failed"
	exit 1
fi
echo "all checks passed"
