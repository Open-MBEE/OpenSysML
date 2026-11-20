#!/usr/bin/env bash
# rebaseline-merge.sh <head-sha>: merges origin/develop into the checked-out branch ($BRANCH) for
# ci:rebaseline, taking develop's side of a conflict on $GENERATED_PATHS (one per line) only.
set -euo pipefail

head_sha=${1:?usage: rebaseline-merge.sh <head-sha>}
: "${GENERATED_PATHS:?GENERATED_PATHS lists the generated paths}" "${BRANCH:?BRANCH names the branch}"
output=${GITHUB_OUTPUT:-/dev/null}

git config user.name "github-actions[bot]"
git config user.email "41898282+github-actions[bot]@users.noreply.github.com"
if [ "$(git rev-parse HEAD)" != "$head_sha" ]; then
	echo "the branch moved since the label was applied ($head_sha); apply the label again" >&2
	exit 1
fi
base=$(git merge-base HEAD origin/develop)
theirs=$(git rev-parse origin/develop)
{
	echo "base=$base"
	echo "ours=$head_sha"
	echo "theirs=$theirs"
} >> "$output"
if [ "$base" = "$theirs" ]; then
	echo "develop is already merged; re-recording on the current head"
elif ! git merge --no-ff --no-commit origin/develop; then
	conflicts=$(git diff --name-only --diff-filter=U)
	generated=$(printf '%s\n' "$GENERATED_PATHS" | sed '/^$/d')
	other=""
	while IFS= read -r file; do
		ok=0
		while IFS= read -r path; do
			case "$file" in "$path"|"$path"/*) ok=1 ;; *) ;; esac  # another generated path may still claim the file
		done <<< "$generated"
		case "$file" in */testdata/*.golden|*/testdata/*.golden.*|*/testdata/*.sha256) ;; */testdata/*.sysml|*/testdata/*.kerml|*/testdata/*.ttl) ok=0 ;; *) ;; esac  # any other file keeps the generated-paths verdict
		[ "$ok" = 1 ] || other="$other$file"$'\n'
	done <<< "$conflicts"
	if [ -n "$other" ]; then
		printf 'conflicts outside the generated paths; resolve them by hand:\n%s' "$other" >&2
		exit 1
	fi
	while IFS= read -r file; do
		if git cat-file -e "origin/develop:$file" 2>/dev/null; then
			git checkout --theirs -- "$file" && git add -- "$file"
		else
			git rm -q -- "$file"
		fi
	done <<< "$conflicts"
	git commit -q -m "Merge origin/develop into $BRANCH" \
		-m "Generated paths in conflict took develop's side; re-recorded by the ci:rebaseline workflow."
else
	git commit -q -m "Merge origin/develop into $BRANCH"
fi
echo "merged=$(git rev-parse HEAD)" >> "$output"
