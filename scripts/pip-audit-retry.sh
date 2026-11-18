#!/usr/bin/env bash

# Runs pip-audit so that a slow or failed answer from PyPI is retried: its vulnerability feed
# and its dependency resolution both ask PyPI, and one read that takes 15 seconds ends the audit.
set -euo pipefail

[[ $# -ge 2 ]] || { echo "usage: $0 <pip-audit> <pip-audit arguments>..." >&2; exit 2; }
tool=$1
shift

attempts=${PIP_AUDIT_ATTEMPTS:-4}
bound=${PIP_AUDIT_TIMEOUT_SECONDS:-300}
read_timeout=${PIP_AUDIT_READ_SECONDS:-60}
# The pip that pip-audit resolves with reads its timeout and retries from the environment.
export PIP_TIMEOUT=${PIP_TIMEOUT:-$read_timeout} PIP_RETRIES=${PIP_RETRIES:-8}

# GNU timeout bounds an attempt where there is one (gtimeout on macOS with coreutils); without
# it, as on a stock macOS, the attempt runs under the read timeouts alone.
bounded=()
for command in timeout gtimeout; do
  if command -v "$command" >/dev/null 2>&1; then
    bounded=("$command" -k 10 "$bound")
    break
  fi
done

log=$(mktemp)
trap 'rm -f "$log"' EXIT
for ((attempt = 1; attempt <= attempts; attempt++)); do
  status=0
  ${bounded[@]+"${bounded[@]}"} "$tool" --timeout "$read_timeout" "$@" 2>&1 | tee "$log" || status=$?
  ((status != 0)) || exit 0
  # A finding is the audit's answer, not a failure to get one: it is reported at once.
  if grep -Eq 'Found [0-9]+ known vulnerabilit' "$log"; then
    exit "$status"
  fi
  echo "pip-audit did not complete (attempt $attempt of $attempts, exit $status)" >&2
  ((attempt == attempts)) || sleep $((15 * attempt))
done
echo "error: pip-audit $* did not complete in $attempts attempts" >&2
exit "$status"
