#!/usr/bin/env bash
# Exercises scripts/upstream-drift-issue.sh against a recorded gh: every call
# the script makes is logged, and the one open issue the fake tracker holds is
# scripted per case. No network, no token.
set -euo pipefail

script="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/upstream-drift-issue.sh"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

mkdir -p "$work/bin"
cat >"$work/bin/gh" <<'FAKE'
#!/usr/bin/env bash
# A gh that answers from files: OPEN_ISSUE (number or empty), ISSUE_BODY (the
# body of that issue), and appends every invocation to GH_LOG.
printf '%s\n' "$*" >>"$GH_LOG"
case "$1 $2" in
"label create") exit 0 ;;
"issue list") printf '%s\n' "$(cat "$OPEN_ISSUE")" ;;
"issue view") cat "$ISSUE_BODY" ;;
"issue create") echo "https://example.invalid/issues/7" ;;
"issue edit" | "issue comment" | "issue close") exit 0 ;;
*) echo "fake gh: unexpected $*" >&2; exit 99 ;;
esac
FAKE
chmod +x "$work/bin/gh"
export PATH="$work/bin:$PATH" GH_LOG="$work/gh.log" OPEN_ISSUE="$work/open" ISSUE_BODY="$work/body"

report="$work/drift.txt"
printf '%s\n' \
	"Pin: 2026-09 abc https://example.invalid/pilot.git" \
	"MOVED: pilot implementation: release 2026-09.1 is published, the pin is 2026-09" \
	"Upstream moved; re-pin deliberately, nothing was changed here." >"$report"
run_url="https://example.invalid/actions/runs/1"

failed=0
ok() { echo "ok   $*"; }
fail() {
	echo "FAIL $*" >&2
	failed=1
}
expect_log() {
	local what=$1 pattern=$2
	if grep -qE -- "$pattern" "$GH_LOG"; then ok "$what"; else
		fail "$what: no gh call matching $pattern; calls were:"
		sed 's/^/     /' "$GH_LOG" >&2
	fi
}
reject_log() {
	local what=$1 pattern=$2
	if grep -qE -- "$pattern" "$GH_LOG"; then
		fail "$what: unexpected gh call matching $pattern"
	else ok "$what"; fi
}
reset() {
	: >"$GH_LOG"
	printf '%s' "${1:-}" >"$OPEN_ISSUE"
	: >"$ISSUE_BODY"
}

# 1. Movement with no open issue: the label is ensured and an issue opened with
#    the newest release in its title and the report in its body.
reset ""
out="$("$script" moved "$report" "$run_url")"
expect_log "label is created with --force" '^label create upstream-drift --force'
expect_log "issue is opened with the release in the title" '^issue create --title Upstream drift: pilot release 2026-09.1 is published past the pin --label upstream-drift --body '
expect_log "issue body carries the report" 'MOVED: pilot implementation: release 2026-09.1 is published'
expect_log "issue body links the run" "$run_url"
if [[ "$out" == "opened https://example.invalid/issues/7" ]]; then ok "reports the opened issue"; else fail "unexpected output: $out"; fi

# 2. Movement with an open issue carrying the same report: the body is
#    refreshed with the new run, no comment and no second issue.
reset 7
"$script" moved "$report" "https://example.invalid/actions/runs/0" >/dev/null
awk '/^issue edit/ {found=1} END {exit !found}' "$GH_LOG" || fail "setup: no edit recorded"
# Reconstruct the body the script wrote from the edit call's --body argument.
{
	printf '%s\n' '<!-- upstream-drift-report -->' 'x' '' 'x' '' '```'
	cat "$report"
	printf '%s\n' '```'
} >"$ISSUE_BODY"
: >"$GH_LOG"
out="$("$script" moved "$report" "$run_url")"
reject_log "no second issue for a repeated report" '^issue create'
reject_log "no comment for a repeated report" '^issue comment'
expect_log "body is refreshed" '^issue edit 7 --body '
expect_log "refreshed body links the current run" "$run_url"
if [[ "$out" == *"already carries this report"* ]]; then ok "says the report is unchanged"; else fail "unexpected output: $out"; fi

# 3. Movement with an open issue carrying a different report: the title and
#    body are rewritten and the change is commented so subscribers hear of it.
reset 7
printf '%s\n' '<!-- upstream-drift-report -->' '' '```' 'MOVED: something else' '```' >"$ISSUE_BODY"
out="$("$script" moved "$report" "$run_url")"
reject_log "no second issue for a changed report" '^issue create'
expect_log "title follows the newest release" '^issue edit 7 --title Upstream drift: pilot release 2026-09.1 is published past the pin --body '
expect_log "change is commented with the report" '^issue comment 7 --body The \[latest run\].*reports a change'
if [[ "$out" == *"updated issue #7"* ]]; then ok "says the issue was updated"; else fail "unexpected output: $out"; fi

# 4. Current pin with an open issue: it is closed with the report; without one,
#    nothing is touched.
reset 7
printf '%s\n' "Upstream is where the pin says." >"$report"
out="$("$script" current "$report" "$run_url")"
expect_log "open issue is closed with a comment" '^issue close 7 --comment The \[latest run\].*finds upstream where the pin says'
if [[ "$out" == "closed issue #7" ]]; then ok "reports the closed issue"; else fail "unexpected output: $out"; fi
reset ""
out="$("$script" current "$report" "$run_url")"
reject_log "nothing is closed without an open issue" '^issue (close|create|edit|comment)'
if [[ "$out" == *"nothing to close"* ]]; then ok "says there was nothing to close"; else fail "unexpected output: $out"; fi

# 5. A report without the pilot-release line still gets a title; bad arguments
#    are refused before gh is consulted.
reset ""
printf '%s\n' "MOVED: OMG SysML page lists versions this script has not recorded: 2.1" >"$report"
"$script" moved "$report" "$run_url" >/dev/null
expect_log "generic title when no release is named" '^issue create --title Upstream drift: the pilot or specification moved past the pin --label'
: >"$GH_LOG"
if "$script" bogus "$report" "$run_url" 2>/dev/null; then fail "bogus verdict accepted"; else ok "bogus verdict is refused"; fi
if "$script" moved "$work/absent.txt" "$run_url" 2>/dev/null; then fail "absent report accepted"; else ok "absent report is refused"; fi
if [[ ! -s "$GH_LOG" ]]; then ok "refusals consult no gh"; else fail "refusals called gh"; fi

if [[ "$failed" -ne 0 ]]; then
	echo "some upstream-drift-issue cases failed" >&2
	exit 1
fi
echo "all upstream-drift-issue cases passed"
