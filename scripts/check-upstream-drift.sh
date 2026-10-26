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

# The OMG specification pages, and the version entries they listed when this
# baseline was recorded. A new entry (a formal 2.1, a KerML 1.1 beta) is reported;
# extend the lists once it has been read.
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

# newest_tag <repo>: the highest release tag by version order, from the remote.
newest_tag() {
	git ls-remote --tags --refs "$1" | sed 's|.*refs/tags/||' | grep -E '^[0-9]{4}-[0-9]{2}$' | sort -V | tail -n 1
}

# tag_commit <repo> <tag>: the commit a tag resolves to on the remote.
tag_commit() {
	git ls-remote "$1" "refs/tags/$2^{}" "refs/tags/$2" | awk 'NR==1 {print $1}'
}

# check_repo <name> <repo> <pinned commit>: reports the pinned tag's state and
# leaves the newest release tag in NEWEST_TAG.
NEWEST_TAG=
check_repo() {
	local name=$1 repo=$2 pinned_commit=$3 newest resolved
	resolved="$(tag_commit "$repo" "$PILOT_TAG")"
	if [[ -z "$resolved" ]]; then
		movement "$name: the pinned tag $PILOT_TAG no longer exists at $repo"
	elif [[ "$resolved" != "$pinned_commit" ]]; then
		movement "$name: tag $PILOT_TAG resolves to $resolved, the pin says $pinned_commit"
	else
		report "$name: tag $PILOT_TAG still resolves to $pinned_commit"
	fi
	newest="$(newest_tag "$repo")"
	if [[ -z "$newest" ]]; then
		movement "$name: no release tag found at $repo"
	elif [[ "$newest" != "$PILOT_TAG" && "$(printf '%s\n%s\n' "$PILOT_TAG" "$newest" | sort -V | tail -n 1)" == "$newest" ]]; then
		movement "$name: release $newest is published, the pin is $PILOT_TAG"
	else
		report "$name: $PILOT_TAG is the newest release"
	fi
	NEWEST_TAG="$newest"
}

# spec_docs_changed <release tag>: compares the blob ids of the specification
# PDFs between the pinned release commit and the given tag, cloning nothing but
# the trees.
spec_docs_changed() {
	local tag=$1 dir pinned_id tag_id changed=0
	dir="$(mktemp -d)"
	trap 'rm -rf "$dir"' RETURN
	git init -q "$dir"
	git -C "$dir" remote add origin "$PILOT_RELEASE_REPO"
	git -C "$dir" fetch -q --filter=tree:0 --depth 1 origin "refs/tags/$tag:refs/tags/$tag" "$PILOT_RELEASE_COMMIT"
	for doc in "${SPEC_DOCS[@]}"; do
		pinned_id="$(git -C "$dir" rev-parse -q --verify "$PILOT_RELEASE_COMMIT:$doc" 2>/dev/null || true)"
		tag_id="$(git -C "$dir" rev-parse -q --verify "refs/tags/$tag:$doc" 2>/dev/null || true)"
		if [[ "$pinned_id" == "$tag_id" ]]; then
			report "  $doc: unchanged"
		else
			report "  $doc: ${pinned_id:-absent} -> ${tag_id:-absent}"
			changed=1
		fi
	done
	return $changed
}

# omg_versions <page url> <language>: the version entries the OMG page links.
omg_versions() {
	curl -fsSL --max-time 60 "$1" | grep -oE "spec/$2/[0-9]+\.[0-9]+(/Beta[0-9]+)?" | sed "s|spec/$2/||" | sort -u
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
	if spec_docs_changed "$newest_release"; then
		report "  the specification PDFs are unchanged: a re-pin moves the pilot, not the clause numbers"
	else
		movement "the specification PDFs changed between $PILOT_TAG and $newest_release: a re-pin must re-read the clause numbers docs/project/spec-compliance.md cites"
	fi
fi
check_omg SysML "$OMG_SYSML_PAGE" "$OMG_SYSML_KNOWN"
check_omg KerML "$OMG_KERML_PAGE" "$OMG_KERML_KNOWN"

if [[ "$moved" -ne 0 ]]; then
	report "Upstream moved; re-pin deliberately, nothing was changed here."
	exit 1
fi
report "Upstream is where the pin says."
