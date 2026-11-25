#!/usr/bin/env bash
# Opt-in A/B timing of the serial and bounded gate over two shards; not a CI gate.
set -euo pipefail

cd "$(dirname "$0")/.."

base=${GATE_TIMING_TMP:-$HOME/.cache/opensysml/gate-timing}
mkdir -p "$base/tmp" "$base/temp"
export GOMAXPROCS="${GOMAXPROCS:-2}"
export GOCACHE="${GOCACHE:-$HOME/.cache/go-build}"
export GOTMPDIR="$base/tmp"
export TMPDIR="$base/temp"

echo "date: $(date -u '+%Y-%m-%dT%H:%M:%SZ')"
echo "host: $(nproc) CPUs, $(awk '/MemTotal/{printf "%d MiB", $2/1024}' /proc/meminfo) RAM"
echo "load: $(cat /proc/loadavg)"
echo "go: $(go version)"
echo "GOMAXPROCS=$GOMAXPROCS GOCACHE=$GOCACHE GOTMPDIR=$GOTMPDIR TMPDIR=$TMPDIR"

# Compile the two shards' test binaries once so both runs start from the same cache.
go build ./...
go test -run '^$' -count=1 ./tests/grpc ./internal/frontend/grpc

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
