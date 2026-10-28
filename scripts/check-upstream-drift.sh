#!/usr/bin/env bash
# Reports movement upstream of the pin in scripts/pilot-pin.sh: a newer release
# of the pilot implementation or of the SysML-v2-Release repository, a pinned tag
# that no longer resolves to the pinned commit, specification PDFs that differ
# between the pinned release and the newest one, and OMG specification pages
# that list a version this script has not seen. It changes nothing: a nonzero
# exit is the signal to re-pin deliberately (docs/project/pilot-corpora.md
# describes how), never to move the pin here.
set -euo pipefail

# shellcheck source=scripts/pilot-pin.sh
. "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/pilot-pin.sh"

# The specification documents the pin ships; a release that changes any of them
# moves the clause numbers docs/project/spec-compliance.md cites.
SPEC_DOCS=(
	'doc/1-Kernel_Modeling_Language.pdf'
	'doc/2a-OMG_Systems_Modeling_Language.pdf'
	'doc/2b-SysML_v1_to_v2_Transformation.pdf'
	'doc/3-Systems_Modeling_API_and_Services.pdf'
)

# The OMG specification pages and the version entries they listed when recorded; a new
# entry (a formal 2.1, a KerML 1.1 beta) is reported — extend the lists once it is read.
OMG_SYSML_PAGE="${OMG_SYSML_PAGE:-https://www.omg.org/spec/SysML/}"
OMG_KERML_PAGE="${OMG_KERML_PAGE:-https://www.omg.org/spec/KerML/}"
OMG_SYSML_KNOWN='1.0 1.1 1.2 1.3 1.4 1.5 1.6 1.7 2.0 2.0/Beta4'
OMG_KERML_KNOWN='1.0 1.0/Beta4'

moved=0
report() { printf '%s\n' "$*"; }
movement() {
	moved=1
	report "MOVED: $*"
}

# newest_tag <repo>: the highest YYYY-MM(.N) release tag on the remote (point releases
# sort after their base); fails when the remote cannot be listed, empty when it has none.
newest_tag() {
	local tags
	tags="$(git ls-remote --tags --refs "$1")" || return 1
	awk -F'refs/tags/' 'NF > 1 {print $2}' <<<"$tags" | grep -E '^[0-9]{4}-[0-9]{2}(\.[0-9]+)?$' | sort -V | tail -n 1 || true
}

# tag_commit <repo> <tag>: the commit a tag resolves to (the peeled "^{}" row of an
# annotated tag); fails when the remote cannot be listed, empty when the tag is gone.
tag_commit() {
	local repo=$1 tag=$2 rows
	rows="$(git ls-remote "$repo" "refs/tags/$tag^{}" "refs/tags/$tag")" || return 1
	awk -v t="refs/tags/$tag" '{c[$2]=$1} END {print ((t "^{}") in c) ? c[t "^{}"] : c[t]}' <<<"$rows"
}

# check_repo <name> <repo> <pinned commit>: reports the pinned tag's state and
# leaves the newest release tag in NEWEST_TAG.
NEWEST_TAG=
check_repo() {
	local name=$1 repo=$2 pinned_commit=$3 newest resolved
	NEWEST_TAG=""
	if ! resolved="$(tag_commit "$repo" "$PILOT_TAG")"; then
		movement "$name: tags could not be listed from $repo; nothing was checked there"
		return
	fi
	if [[ -z "$resolved" ]]; then
		movement "$name: the pinned tag $PILOT_TAG no longer exists at $repo"
	elif [[ "$resolved" != "$pinned_commit" ]]; then
		movement "$name: tag $PILOT_TAG resolves to $resolved, the pin says $pinned_commit"
	else
		report "$name: tag $PILOT_TAG still resolves to $pinned_commit"
	fi
	if ! newest="$(newest_tag "$repo")"; then
		movement "$name: release tags could not be listed from $repo"
		return
	fi
	if [[ -z "$newest" ]]; then
		movement "$name: no release tag found at $repo"
	elif [[ "$newest" != "$PILOT_TAG" && "$(printf '%s\n%s\n' "$PILOT_TAG" "$newest" | sort -V | tail -n 1)" == "$newest" ]]; then
		movement "$name: release $newest is published, the pin is $PILOT_TAG"
	else
		report "$name: $PILOT_TAG is the newest release"
	fi
	NEWEST_TAG="$newest"
}

# spec_docs_changed <release tag>: compares the specification PDFs' blob ids between the
# pinned release commit and the tag (trees only): 0 unchanged, 1 changed, 2 unreadable.
spec_docs_changed() {
	local tag=$1 dir pinned_id tag_id verdict=0
	dir="$(mktemp -d)"
	trap 'rm -rf "$dir"' RETURN
	git init -q "$dir"
	git -C "$dir" remote add origin "$PILOT_RELEASE_REPO"
	if ! git -C "$dir" fetch -q --filter=tree:0 --depth 1 origin "refs/tags/$tag:refs/tags/$tag" "$PILOT_RELEASE_COMMIT"; then
		report "  could not fetch $tag and $PILOT_RELEASE_COMMIT from $PILOT_RELEASE_REPO"
		return 2
	fi
	for doc in "${SPEC_DOCS[@]}"; do
		if ! pinned_id="$(git -C "$dir" rev-parse -q --verify "$PILOT_RELEASE_COMMIT:$doc" 2>/dev/null)"; then
			report "  $doc: unreadable at the pinned release commit"
			verdict=2
		elif ! tag_id="$(git -C "$dir" rev-parse -q --verify "refs/tags/$tag:$doc" 2>/dev/null)"; then
			report "  $doc: $pinned_id -> absent in $tag"
			[[ $verdict -eq 0 ]] && verdict=1
		elif [[ "$pinned_id" == "$tag_id" ]]; then
			report "  $doc: unchanged"
		else
			report "  $doc: $pinned_id -> $tag_id"
			[[ $verdict -eq 0 ]] && verdict=1
		fi
	done
	return $verdict
}

# omg_versions <page url> <language>: the version entries the OMG page links.
omg_versions() {
	local page=$1 language=$2
	curl -fsSL --max-time 60 "$page" | grep -oE "spec/$language/[0-9]+\.[0-9]+(/Beta[0-9]+)?" | sed "s|spec/$language/||" | sort -u
}

check_omg() {
	local name=$1 page=$2 known=$3 listed new
	if ! listed="$(omg_versions "$page" "$name")"; then
		movement "OMG $name page $page could not be read"
		return
	fi
	new="$(comm -13 <(tr ' ' '\n' <<<"$known" | sort -u) <(printf '%s\n' "$listed"))"
	if [[ -n "$new" ]]; then
		movement "OMG $name page lists versions this script has not recorded: $(tr '\n' ' ' <<<"$new")"
	else
		report "OMG $name page lists nothing beyond: $known"
	fi
}

report "Pin: $(pilot_pin) (release repository $PILOT_RELEASE_REPO at $PILOT_RELEASE_COMMIT, artifact $PILOT_ARTIFACT_VERSION)"
check_repo 'pilot implementation' "$PILOT_REPO" "$PILOT_COMMIT"
check_repo 'release repository' "$PILOT_RELEASE_REPO" "$PILOT_RELEASE_COMMIT"
newest_release="$NEWEST_TAG"
if [[ -n "$newest_release" && "$newest_release" != "$PILOT_TAG" ]]; then
	report "Specification documents, $PILOT_TAG -> $newest_release:"
	spec_docs_changed "$newest_release" && verdict=0 || verdict=$?
	case $verdict in
	0) report "  the specification PDFs are unchanged: a re-pin moves the pilot, not the clause numbers" ;;
	1) movement "the specification PDFs changed between $PILOT_TAG and $newest_release: a re-pin must re-read the clause numbers docs/project/spec-compliance.md cites" ;;
	*) movement "the specification PDFs of $PILOT_TAG and $newest_release could not be compared; read them by hand before re-pinning" ;;
	esac
fi
check_omg SysML "$OMG_SYSML_PAGE" "$OMG_SYSML_KNOWN"
check_omg KerML "$OMG_KERML_PAGE" "$OMG_KERML_KNOWN"

if [[ "$moved" -ne 0 ]]; then
	report "Upstream moved; re-pin deliberately, nothing was changed here."
	exit 1
fi
report "Upstream is where the pin says."
