#!/usr/bin/env bash
# Exercises the exit codes of scripts/check-upstream-drift.sh, which the
# Upstream drift workflow reads: 0 current, 1 moved, 2 a source unread. Runs
# against throwaway local repositories and unreachable page URLs, without the
# network.
set -euo pipefail

script="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/check-upstream-drift.sh"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

failed=0
ok() { echo "ok   $*"; }
fail() {
	echo "FAIL $*" >&2
	failed=1
}

# A release-shaped repository: the pinned tag at one commit, with the four
# specification documents, and optionally a newer release tag.
make_repo() {
	local dir=$1
	git init -q -b main "$dir"
	git -C "$dir" -c user.name=t -c user.email=t@example.invalid commit -q --allow-empty -m pinned
	git -C "$dir" tag 2026-09
	git -C "$dir" config uploadpack.allowFilter true
	git -C "$dir" config uploadpack.allowAnySHA1InWant true
}
pilot="$work/pilot.git"
release="$work/release.git"
make_repo "$pilot"
make_repo "$release"
pinned_pilot="$(git -C "$pilot" rev-parse 2026-09)"
pinned_release="$(git -C "$release" rev-parse 2026-09)"
page="$work/page.html"
printf '<a href="/spec/SysML/2.0/Beta4">2.0 Beta4</a><a href="/spec/KerML/1.0/Beta4">1.0 Beta4</a>\n' >"$page"

run() {
	local status=0
	PILOT_TAG=2026-09 PILOT_COMMIT="$pinned_pilot" PILOT_RELEASE_COMMIT="$pinned_release" \
		PILOT_REPO="file://$pilot" PILOT_RELEASE_REPO="file://$release" \
		OMG_SYSML_PAGE="${SYSML_PAGE:-file://$page}" OMG_KERML_PAGE="${KERML_PAGE:-file://$page}" \
		"$script" >"$work/out.txt" 2>&1 || status=$?
	echo "$status"
}
expect() {
	local what=$1 want=$2 got=$3 pattern=$4
	if [[ "$got" == "$want" ]] && grep -qE -- "$pattern" "$work/out.txt"; then ok "$what"; else
		fail "$what: exit $got (want $want); output:"
		sed 's/^/     /' "$work/out.txt" >&2
	fi
}

# 1. Everything where the pin says: exit 0.
expect "pin current exits 0" 0 "$(run)" '^Upstream is where the pin says\.$'

# 2. A newer release tag upstream: exit 1 and the movement named.
git -C "$pilot" -c user.name=t -c user.email=t@example.invalid commit -q --allow-empty -m newer
git -C "$pilot" tag 2026-09.1
expect "newer release exits 1" 1 "$(run)" '^MOVED: pilot implementation: release 2026-09.1 is published, the pin is 2026-09$'
git -C "$pilot" tag -d 2026-09.1 >/dev/null

# 3. The pinned tag moved to another commit: exit 1.
git -C "$release" -c user.name=t -c user.email=t@example.invalid commit -q --allow-empty -m retag
git -C "$release" tag -f 2026-09 >/dev/null
expect "moved tag exits 1" 1 "$(run)" '^MOVED: release repository: tag 2026-09 resolves to [0-9a-f]{40}, the pin says '"$pinned_release"'$'
git -C "$release" tag -f 2026-09 "$pinned_release" >/dev/null

# 4. An OMG page listing an unrecorded version: exit 1.
printf '<a href="/spec/SysML/2.1">2.1</a>\n' >"$work/newer.html"
expect "unrecorded OMG version exits 1" 1 "$(SYSML_PAGE="file://$work/newer.html" run)" '^MOVED: OMG SysML page lists versions this script has not recorded: 2.1'

# 5. A source that cannot be read, with no movement found: exit 2, not 1.
expect "unreadable page exits 2" 2 "$(SYSML_PAGE="file://$work/absent.html" run)" '^UNREAD: OMG SysML page .* could not be read$'
expect "unreadable page is not reported as movement" 2 "$(SYSML_PAGE="file://$work/absent.html" run)" '^Upstream could not be fully read; nothing is known about movement\.$'
if grep -q '^MOVED:' "$work/out.txt"; then fail "unreadable page reported MOVED"; else ok "unreadable page reports no MOVED line"; fi

# 6. Movement and an unread source together: movement wins, exit 1.
git -C "$pilot" tag 2026-09.1
expect "movement beside an unread source exits 1" 1 "$(SYSML_PAGE="file://$work/absent.html" run)" '^UNREAD: OMG SysML page'
if grep -q '^MOVED: pilot implementation: release 2026-09.1' "$work/out.txt"; then ok "the movement is still reported"; else fail "movement missing beside an unread source"; fi

if [[ "$failed" -ne 0 ]]; then
	echo "some check-upstream-drift cases failed" >&2
	exit 1
fi
echo "all check-upstream-drift cases passed"
