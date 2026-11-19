#!/usr/bin/env bash

# Checks scripts/maven-sbom.sh against a stand-in mvn that plays Maven Central's HTTP 429, so the
# fallback to the mirror, and the build error that must not be retried, run on every pull request.
set -euo pipefail

script=$(cd "$(dirname "$0")" && pwd)/maven-sbom.sh
mirror=$(cd "$(dirname "$0")" && pwd)/maven-central-mirror-settings.xml
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
stubs=$work/bin
mkdir -p "$stubs"

cat >"$stubs/sleep" <<'STUB'
#!/usr/bin/env bash
exit 0
STUB
# FAKE_MVN_CENTRAL=down refuses every call that names no mirror, FAKE_MVN_MIRROR=down every call
# that names one, and FAKE_MVN_REFUSALS the first calls whichever they ask; FAKE_MVN_BROKEN fails
# as a pom.xml Maven cannot build does.
cat >"$stubs/mvn" <<'STUB'
#!/usr/bin/env bash
echo "$*" >>"$FAKE_MVN_LOG"
refuse() {
  echo "[ERROR] Plugin org.cyclonedx:cyclonedx-maven-plugin:2.9.1 or one of its dependencies could not be resolved:" >&2
  echo "[ERROR]     Could not find artifact org.cyclonedx:cyclonedx-maven-plugin:jar:2.9.1 in central ($1)" >&2
  exit 1
}
if [[ -n ${FAKE_MVN_BROKEN:-} ]]; then
  echo "[ERROR] Non-parseable POM pom.xml: unexpected markup" >&2
  exit 1
fi
(($(grep -c . "$FAKE_MVN_LOG") > ${FAKE_MVN_REFUSALS:-0})) || refuse "https://repo.maven.apache.org/maven2"
if [[ " $* " == *" -s "* ]]; then
  [[ ${FAKE_MVN_MIRROR:-up} == up ]] || refuse "the mirror"
else
  [[ ${FAKE_MVN_CENTRAL:-up} == up ]] || refuse "https://repo.maven.apache.org/maven2"
fi
STUB
chmod +x "$stubs"/*

failures=0
fail() {
  echo "FAIL $*" >&2
  failures=$((failures + 1))
}

# run_case <name> <expected exit> [VAR=value ...]: writes the SBOM of a tree through the script.
run_case() {
  local name=$1 expected=$2 actual=0
  shift 2
  mkdir -p "$work/$name"
  env PATH="$stubs:$PATH" FAKE_MVN_LOG="$work/$name/log" "$@" \
    bash "$script" editors/mdk 2.9.1 >"$work/$name/out" 2>&1 || actual=$?
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
  [[ $actual == "$expected" ]] || fail "$name: mvn ran $actual times, expected $expected"
}
expect_call() {
  local name=$1 number=$2 expected=$3 actual
  actual=$(sed -n "${number}p" "$work/$name/log")
  [[ $actual == "$expected" ]] || fail "$name: call $number was [mvn $actual], expected [mvn $expected]"
}
mentions() {
  local name=$1 text=$2
  grep -q -- "$text" "$work/$name/out" || fail "$name: output lacks \"$text\""
}

goal="-B -q -f editors/mdk/pom.xml org.cyclonedx:cyclonedx-maven-plugin:2.9.1:makeAggregateBom -DoutputFormat=json -DoutputName=bom -DincludeTestScope=false"

# Maven Central answers: one run, with no settings file and no forced update.
if run_case clean 0; then
  expect_calls clean 1
  expect_call clean 1 "$goal"
fi

# Maven Central refuses for the whole job: the second attempt asks the mirror and passes.
if run_case central-down 0 FAKE_MVN_CENTRAL=down; then
  expect_calls central-down 2
  expect_call central-down 2 "-U -s $mirror $goal"
  mentions central-down "(attempt 1 of 4, exit 1)"
fi

# The mirror is down and Central recovers: the third attempt asks Central again.
if run_case mirror-down 0 FAKE_MVN_REFUSALS=1 FAKE_MVN_MIRROR=down; then
  expect_calls mirror-down 3
  expect_call mirror-down 3 "-U $goal"
fi

# Neither answers: the attempts are bounded and the audit fails rather than scanning no SBOM.
if run_case outage 1 FAKE_MVN_CENTRAL=down FAKE_MVN_MIRROR=down MAVEN_SBOM_ATTEMPTS=3; then
  expect_calls outage 3
  mentions outage "no SBOM of editors/mdk in 3 attempts"
fi

# A pom.xml Maven cannot build fails on the first attempt.
if run_case broken 1 FAKE_MVN_BROKEN=1; then
  expect_calls broken 1
  mentions broken "Non-parseable POM"
fi

actual=0
bash "$script" editors/mdk >/dev/null 2>&1 || actual=$?
if [[ $actual == 2 ]]; then echo "ok   usage: exit 2"; else fail "usage: exit $actual, expected 2"; fi

if ((failures > 0)); then
  echo "$failures check(s) failed" >&2
  exit 1
fi
echo "all checks passed"
