#!/usr/bin/env bash
# Opt-in A/B timing of the serial and bounded gate over two shards; not a CI gate.
set -euo pipefail

cd "$(dirname "$0")/.."

die() {
  printf 'gate-timing: %s\n' "$*" >&2
  exit 2
}

# Every directory this script creates stays under the approved home cache.
if [[ -z "${HOME:-}" || "$HOME" != /* ]]; then
  die 'HOME must be set to an absolute path to resolve the approved root'
fi
approved_root="$HOME/.cache/opensysml"

rel=${GATE_TIMING_TMP:-gate-timing}
# Validate the whole value before building any path from it: `read` below sees
# only the first line, while mkdir would receive every line of the raw value.
if [[ "$rel" == *[[:cntrl:]]* ]]; then
  die 'GATE_TIMING_TMP must not contain control characters'
fi
if [[ ! "$rel" =~ ^[A-Za-z0-9._/-]+$ ]]; then
  die "GATE_TIMING_TMP has an unsupported component: $rel"
fi
if [[ "$rel" != "${rel#/}" ]]; then
  die "GATE_TIMING_TMP must be relative to $approved_root, got absolute path $rel"
fi
parts=()
IFS='/' read -r -a parts <<< "$rel"
base="$approved_root"
for part in "${parts[@]}"; do
  case "$part" in
    ''|.) continue ;;
    ..) die "GATE_TIMING_TMP must not contain '..': $rel" ;;
    *[!A-Za-z0-9._-]*) die "GATE_TIMING_TMP has an unsupported component: $part" ;;
  esac
  base="$base/$part"
done

# Check every path component for symlinks before mkdir and confirm the real path
# stays inside the approved root.
check_no_symlinks() {
  local path=$1 walked=''
  local components=()
  IFS='/' read -r -a components <<< "$path"
  for part in "${components[@]}"; do
    [[ -z "$part" ]] && continue
    walked+="/$part"
    if [[ -L "$walked" ]]; then
      die "refusing a symlinked path component: $walked"
    fi
  done
}

approved_real=$(realpath -m -- "$approved_root") || die 'cannot resolve the approved root'
check_no_symlinks "$base/tmp"
check_no_symlinks "$base/temp"
mkdir -p "$base/tmp" "$base/temp"
resolved_base=$(realpath -e -- "$base") || die 'cannot resolve the temporary directory'
case "$resolved_base" in
  "$approved_real"/*) ;;
  *) die "temporary directory resolved outside $approved_root: $resolved_base" ;;
esac
check_no_symlinks "$base/tmp"
check_no_symlinks "$base/temp"

export GOMAXPROCS="${GOMAXPROCS:-2}"
export GOCACHE="${GOCACHE:-$HOME/.cache/go-build}"
export GOTMPDIR="$base/tmp"
export TMPDIR="$base/temp"

# Both arms must execute tests: without this, the first arm caches successful
# test results and the second replays them, mixing execution with cache hits.
if [[ -n "${GOFLAGS:-}" ]]; then
  export GOFLAGS="$GOFLAGS -count=1"
else
  export GOFLAGS='-count=1'
fi

echo "date: $(date -u '+%Y-%m-%dT%H:%M:%SZ')"
echo "host: $(nproc) CPUs, $(awk '/MemTotal/{printf "%d MiB", $2/1024}' /proc/meminfo) RAM"
echo "load: $(cat /proc/loadavg)"
echo "go: $(go version)"
echo "GOMAXPROCS=$GOMAXPROCS GOCACHE=$GOCACHE GOTMPDIR=$GOTMPDIR TMPDIR=$TMPDIR GOFLAGS=$GOFLAGS"

# Regenerate the embedded-library snapshot first: each timed shard runs make's
# snapshot prerequisite, which adds a generated Go file to the packages below.
make --no-print-directory stdlib-snapshot

# Compile both timed arms' exact test sets via the make target, with -pgo=off and
# the same shard flags as the measured runs.
make --no-print-directory test-quick SHARDS="grpc rest" \
  SKIP_STDLIB_SNAPSHOT=1 TEST_SHARD_COMPILE_ONLY=1

serial='serial make test-quick SHARDS="grpc rest"'
runner='runner gate-runner -tier quick -concurrency 2'
set +e
echo "--- $serial ---"
/usr/bin/time -f "$serial: %e s %M KB" make test-quick SHARDS="grpc rest"
serial_status=$?
echo "--- $runner ---"
/usr/bin/time -f "$runner: %e s %M KB" go run -C tools ./cmd/gate-runner -tier quick -shards grpc,rest -concurrency 2
runner_status=$?
echo "statuses: serial=$serial_status runner=$runner_status (this script records timings, it is not a gate)"
exit 0
