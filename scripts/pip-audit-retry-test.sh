#!/usr/bin/env bash

# Checks scripts/pip-audit-retry.sh against a stand-in pip-audit that plays PyPI's read timeout,
# so the retry, and the finding that must not be retried into a pass, run on every pull request.
set -euo pipefail

script=$(cd "$(dirname "$0")" && pwd)/pip-audit-retry.sh
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
stubs=$work/bin
mkdir -p "$stubs"

cat >"$stubs/sleep" <<'STUB'
#!/usr/bin/env bash
exit 0
STUB
# FAKE_AUDIT_TIMEOUTS calls fail as a read timeout does; the next reports FAKE_AUDIT_FINDINGS.
cat >"$stubs/pip-audit" <<'STUB'
#!/usr/bin/env bash
echo "$* PIP_TIMEOUT=$PIP_TIMEOUT PIP_RETRIES=$PIP_RETRIES" >>"$FAKE_AUDIT_LOG"
if (($(grep -c . "$FAKE_AUDIT_LOG") <= ${FAKE_AUDIT_TIMEOUTS:-0})); then
  echo "requests.exceptions.ReadTimeout: HTTPSConnectionPool(host='pypi.org', port=443): Read timed out. (read timeout=15)" >&2
  exit 1
fi
if ((${FAKE_AUDIT_FINDINGS:-0} > 0)); then
  echo "Found $FAKE_AUDIT_FINDINGS known vulnerability in 1 package" >&2
  echo "jinja2 2.4.1 PYSEC-2014-8 2.7.2"
  exit 1
fi
echo "No known vulnerabilities found" >&2
STUB
chmod +x "$stubs"/*

failures=0
fail() {
  echo "FAIL $*" >&2
  failures=$((failures + 1))
}

# run_case <name> <expected exit> [VAR=value ...]: audits a requirements file through the script.
run_case() {
  local name=$1 expected=$2 actual=0
  shift 2
  mkdir -p "$work/$name"
  env PATH="$stubs:$PATH" FAKE_AUDIT_LOG="$work/$name/log" "$@" \
    bash "$script" "$stubs/pip-audit" --strict -r requirements.txt >"$work/$name/out" 2>&1 || actual=$?
  if [[ $actual != "$expected" ]]; then
    fail "$name: exit $actual, expected $expected"
    sed 's/^/  /' "$work/$name/out" >&2
    return 1
  fi
  echo "ok   $name: exit $actual"
}
expect_calls() {
  local name=$1 expected=$2 actual
  actual=$(grep -c . "$work/$name/log")
  [[ $actual == "$expected" ]] || fail "$name: pip-audit ran $actual times, expected $expected"
}
mentions() {
  local name=$1 text=$2
  grep -q -- "$text" "$work/$name/out" || fail "$name: output lacks \"$text\""
}

# A clean audit runs once, with the longer read timeout for the feed and for pip's resolution.
if run_case clean 0; then
  expect_calls clean 1
  grep -qx -- '--timeout 60 --strict -r requirements.txt PIP_TIMEOUT=60 PIP_RETRIES=8' "$work/clean/log" || fail "clean: pip-audit was run as: $(cat "$work/clean/log")"
fi

# Two read timeouts, then an answer: the third attempt passes.
if run_case slow 0 FAKE_AUDIT_TIMEOUTS=2; then
  expect_calls slow 3
  mentions slow "pip-audit did not complete (attempt 2 of 4, exit 1)"
fi

# A vulnerability fails the audit on the attempt that found it, first or after a timeout.
if run_case finding 1 FAKE_AUDIT_FINDINGS=1; then
  expect_calls finding 1
  mentions finding "Found 1 known vulnerability in 1 package"
  mentions finding "PYSEC-2014-8"
fi
if run_case late-finding 1 FAKE_AUDIT_TIMEOUTS=1 FAKE_AUDIT_FINDINGS=2; then
  expect_calls late-finding 2
fi

# PyPI never answers: the attempts are bounded and the audit fails.
if run_case outage 1 FAKE_AUDIT_TIMEOUTS=9 PIP_AUDIT_ATTEMPTS=3; then
  expect_calls outage 3
  mentions outage "did not complete in 3 attempts"
fi

# A caller's own bounds reach pip-audit and its pip.
if run_case bounds 0 PIP_AUDIT_READ_SECONDS=90 PIP_RETRIES=2; then
  grep -qx -- '--timeout 90 --strict -r requirements.txt PIP_TIMEOUT=90 PIP_RETRIES=2' "$work/bounds/log" || fail "bounds: pip-audit was run as: $(cat "$work/bounds/log")"
fi

# Without GNU timeout on PATH, as on a stock macOS, the audit still runs; with gtimeout it is bounded by it.
bare=$work/bare
mkdir -p "$bare"
for command in bash env grep tee mktemp rm; do
  ln -s "$(command -v "$command")" "$bare/$command"
done
cp "$stubs/sleep" "$stubs/pip-audit" "$bare/"
mkdir -p "$work/no-timeout"
actual=0
env -i PATH="$bare" FAKE_AUDIT_LOG="$work/no-timeout/log" FAKE_AUDIT_TIMEOUTS=1 \
  bash "$script" "$bare/pip-audit" --strict -r requirements.txt >"$work/no-timeout/out" 2>&1 || actual=$?
if [[ $actual == 0 ]]; then
  echo "ok   no-timeout: exit 0"
  expect_calls no-timeout 2
else
  fail "no-timeout: exit $actual, expected 0"
  sed 's/^/  /' "$work/no-timeout/out" >&2
fi
cat >"$bare/gtimeout" <<'STUB'
#!/usr/bin/env bash
echo "$1 $2 $3" >>"$FAKE_TIMEOUT_LOG"
shift 3
exec "$@"
STUB
chmod +x "$bare/gtimeout"
mkdir -p "$work/gtimeout"
actual=0
env -i PATH="$bare" FAKE_AUDIT_LOG="$work/gtimeout/log" FAKE_TIMEOUT_LOG="$work/gtimeout/bound" PIP_AUDIT_TIMEOUT_SECONDS=120 \
  bash "$script" "$bare/pip-audit" --strict -r requirements.txt >"$work/gtimeout/out" 2>&1 || actual=$?
if [[ $actual == 0 && $(cat "$work/gtimeout/bound" 2>/dev/null) == "-k 10 120" ]]; then
  echo "ok   gtimeout: exit 0"
  expect_calls gtimeout 1
else
  fail "gtimeout: exit $actual, bounded by [$(cat "$work/gtimeout/bound" 2>/dev/null)], expected exit 0 under [-k 10 120]"
fi

actual=0
bash "$script" "$stubs/pip-audit" >/dev/null 2>&1 || actual=$?
if [[ $actual == 2 ]]; then echo "ok   usage: exit 2"; else fail "usage: exit $actual, expected 2"; fi

if ((failures > 0)); then
  echo "$failures check(s) failed" >&2
  exit 1
fi
echo "all checks passed"
