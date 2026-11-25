#!/usr/bin/env bash

# Checks scripts/maven-sbom.sh against a stand-in mvn that plays Maven Central's HTTP 429 and its stall, so
# the fallback to the mirror, and the build error that must not be retried, run on every pull request.
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
# The first FAKE_MVN_STALLS calls hang as a transfer that never ends does.
real_sleep=$(command -v sleep)
# FAKE_MVN_CENTRAL=down refuses every call that names no mirror, FAKE_MVN_MIRROR=down every call
# that names one, and FAKE_MVN_REFUSALS the first calls whichever they ask; FAKE_MVN_BROKEN fails
# as a pom.xml Maven cannot build does.
cat >"$stubs/mvn" <<'STUB'
#!/usr/bin/env bash
echo "$*" >>"$FAKE_MVN_LOG"
(($(grep -c . "$FAKE_MVN_LOG") > ${FAKE_MVN_STALLS:-0})) || exec "$FAKE_MVN_SLEEP" 60
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
if [[ " $* " == *" -gs "* ]]; then
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
  env PATH="$stubs:$PATH" FAKE_MVN_LOG="$work/$name/log" FAKE_MVN_SLEEP="$real_sleep" "$@" \
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
  expect_call central-down 2 "-U -gs $mirror $goal"
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

# Maven Central stalls: the attempt is cut at its bound and the second asks the mirror.
if command -v timeout >/dev/null 2>&1 || command -v gtimeout >/dev/null 2>&1; then
  if run_case stall 0 FAKE_MVN_STALLS=1 MAVEN_SBOM_TIMEOUT_SECONDS=1; then
    expect_calls stall 2
    expect_call stall 2 "-U -gs $mirror $goal"
    mentions stall "in 1 seconds (attempt 1 of 4, exit 124)"
  fi
  # Both stall for the whole job: the attempts are bounded and the audit fails.
  if run_case stalled-outage 124 FAKE_MVN_STALLS=9 MAVEN_SBOM_TIMEOUT_SECONDS=1 MAVEN_SBOM_ATTEMPTS=2; then
    expect_calls stalled-outage 2
    mentions stalled-outage "no SBOM of editors/mdk in 2 attempts"
  fi
fi

# An attempt runs under GNU timeout at the bound asked for, and, where there is none, unbounded.
bare=$work/bare
mkdir -p "$bare"
for command in bash env grep tee mktemp rm dirname true; do
  ln -s "$(type -P "$command")" "$bare/$command"
done
cp "$stubs/sleep" "$stubs/mvn" "$bare/"
mkdir -p "$work/no-timeout"
actual=0
env -i PATH="$bare" FAKE_MVN_LOG="$work/no-timeout/log" FAKE_MVN_REFUSALS=1 \
  bash "$script" editors/mdk 2.9.1 >"$work/no-timeout/out" 2>&1 || actual=$?
if [[ $actual == 0 ]]; then
  echo "ok   no-timeout: exit 0"
  expect_calls no-timeout 2
else
  fail "no-timeout: exit $actual, expected 0"
  sed 's/^/  /' "$work/no-timeout/out" >&2
fi
# The stand-in records the bound of every call but the script's probe of what it accepts.
cat >"$bare/gtimeout" <<'STUB'
#!/usr/bin/env bash
[[ ${*: -1} == true ]] || echo "$1 $2 $3" >>"$FAKE_TIMEOUT_LOG"
shift 3
exec "$@"
STUB
chmod +x "$bare/gtimeout"
for case in "gtimeout 90" "gtimeout-asked 45 MAVEN_SBOM_TIMEOUT_SECONDS=45"; do
  read -r name seconds setting <<<"$case"
  mkdir -p "$work/$name"
  actual=0
  env -i PATH="$bare" FAKE_MVN_LOG="$work/$name/log" FAKE_TIMEOUT_LOG="$work/$name/bound" ${setting:+"$setting"} \
    bash "$script" editors/mdk 2.9.1 >"$work/$name/out" 2>&1 || actual=$?
  if [[ $actual == 0 && $(cat "$work/$name/bound" 2>/dev/null) == "-k 10 $seconds" ]]; then
    echo "ok   $name: exit 0"
    expect_call "$name" 1 "$goal"
  else
    fail "$name: exit $actual, bounded by [$(cat "$work/$name/bound" 2>/dev/null)], expected exit 0 under [-k 10 $seconds]"
  fi
done

# A timeout that refuses -k, as BusyBox 1.30's, bounds the attempt without it and reports
# the 143 of what it ended.
rm "$bare/gtimeout"
cat >"$bare/timeout" <<'STUB'
#!/usr/bin/env bash
[[ $1 != -* ]] || { echo "timeout: invalid option -- '${1#-}'" >&2; exit 1; }
[[ ${*: -1} != true ]] || exit 0
echo "$1" >>"$FAKE_TIMEOUT_LOG"
shift
(($(grep -c . "$FAKE_TIMEOUT_LOG") > ${FAKE_TIMEOUT_CUTS:-0})) || exit 143
exec "$@"
STUB
chmod +x "$bare/timeout"
mkdir -p "$work/no-kill"
actual=0
env -i PATH="$bare" FAKE_MVN_LOG="$work/no-kill/log" FAKE_TIMEOUT_LOG="$work/no-kill/bound" FAKE_TIMEOUT_CUTS=1 \
  bash "$script" editors/mdk 2.9.1 >"$work/no-kill/out" 2>&1 || actual=$?
if [[ $actual == 0 && $(tr '\n' ' ' <"$work/no-kill/bound" 2>/dev/null) == "90 90 " ]]; then
  echo "ok   no-kill: exit 0"
  expect_calls no-kill 1
  expect_call no-kill 1 "-U -gs $mirror $goal"
  mentions no-kill "in 90 seconds (attempt 1 of 4, exit 143)"
else
  fail "no-kill: exit $actual, bounded by [$(tr '\n' ' ' <"$work/no-kill/bound" 2>/dev/null)], expected exit 0 under [90 90 ]"
  sed 's/^/  /' "$work/no-kill/out" >&2
fi

actual=0
bash "$script" editors/mdk >/dev/null 2>&1 || actual=$?
if [[ $actual == 2 ]]; then echo "ok   usage: exit 2"; else fail "usage: exit $actual, expected 2"; fi

if ((failures > 0)); then
  echo "$failures check(s) failed" >&2
  exit 1
fi
echo "all checks passed"
