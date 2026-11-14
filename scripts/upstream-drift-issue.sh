#!/usr/bin/env bash
# Carries the report of scripts/check-upstream-drift.sh into the repository's
# issue tracker, so a pin that has gone stale is seen without anyone opening a
# workflow run. One open issue, labelled upstream-drift, tracks the movement:
# the first report opens it, a later report edits its body and comments only
# when the report changed, and the run that finds the pin current closes it.
#
#   scripts/upstream-drift-issue.sh moved   <report file> <run url>
#   scripts/upstream-drift-issue.sh current <report file> <run url>
#
# Needs gh authenticated with issues: write on the repository GH_REPO or the
# checkout names. Never pushes and never re-pins.
set -euo pipefail

LABEL="${UPSTREAM_DRIFT_LABEL:-upstream-drift}"
MARKER='<!-- upstream-drift-report -->'
FENCE='```'

verdict=${1:-}
report_file=${2:-}
run_url=${3:-}
if [[ -z "$verdict" || -z "$report_file" || -z "$run_url" ]]; then
	echo "usage: $0 moved|current <report file> <run url>" >&2
	exit 2
fi
if [[ ! -r "$report_file" ]]; then
	echo "error: $report_file is not readable" >&2
	exit 2
fi
report="$(cat "$report_file")"

repo_args=()
if [[ -n "${GH_REPO:-}" ]]; then
	repo_args=(--repo "$GH_REPO")
fi

open_issue() {
	gh issue list "${repo_args[@]}" --label "$LABEL" --state open --limit 1 --json number --jq '.[0].number // empty'
}

# The body carries the report between the marker and a fence, so a later run
# can read the report it last wrote and tell a repeat from a change.
body() {
	printf '%s\n' "$MARKER" \
		"The weekly [Upstream drift]($run_url) check found the pilot or specification moved past the pin in \`scripts/pilot-pin.sh\`." \
		"" \
		"Re-pin deliberately: docs/project/pilot-corpora.md describes the downloads and gates to re-run. Nothing was changed by the check; this issue closes itself once a run finds the pin current." \
		"" \
		"$FENCE" \
		"$report" \
		"$FENCE"
}

recorded_report() {
	gh issue view "${repo_args[@]}" "$1" --json body --jq '.body' | awk -v fence="$FENCE" '$0 == fence {if (open) exit; open=1; next} open'
}

title() {
	local newest
	newest="$(sed -n 's/^MOVED: pilot implementation: release \([^ ]*\) is published.*/\1/p' <<<"$report" | head -n 1)"
	if [[ -n "$newest" ]]; then
		printf 'Upstream drift: pilot release %s is published past the pin' "$newest"
	else
		printf 'Upstream drift: the pilot or specification moved past the pin'
	fi
}

case "$verdict" in
moved)
	gh label create "${repo_args[@]}" "$LABEL" --force --color D93F0B \
		--description 'The pinned pilot or specification moved upstream; a deliberate re-pin is due' >/dev/null
	number="$(open_issue)"
	if [[ -z "$number" ]]; then
		url="$(gh issue create "${repo_args[@]}" --title "$(title)" --label "$LABEL" --body "$(body)")"
		echo "opened $url"
	elif [[ "$(recorded_report "$number")" == "$report" ]]; then
		gh issue edit "${repo_args[@]}" "$number" --body "$(body)" >/dev/null
		echo "issue #$number already carries this report; body refreshed with the run"
	else
		gh issue edit "${repo_args[@]}" "$number" --title "$(title)" --body "$(body)" >/dev/null
		gh issue comment "${repo_args[@]}" "$number" --body "$(printf 'The [latest run](%s) reports a change:\n\n%s\n%s\n%s' "$run_url" "$FENCE" "$report" "$FENCE")" >/dev/null
		echo "updated issue #$number with a changed report"
	fi
	;;
current)
	number="$(open_issue)"
	if [[ -z "$number" ]]; then
		echo "no open $LABEL issue; nothing to close"
	else
		gh issue close "${repo_args[@]}" "$number" --comment "$(printf 'The [latest run](%s) finds upstream where the pin says:\n\n%s\n%s\n%s' "$run_url" "$FENCE" "$report" "$FENCE")" >/dev/null
		echo "closed issue #$number"
	fi
	;;
*)
	echo "error: verdict must be moved or current, got $verdict" >&2
	exit 2
	;;
esac
