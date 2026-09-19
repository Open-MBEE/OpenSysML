#!/usr/bin/env bash
# Regenerate cmd/*/default.pgo: one merged CPU profile of the test suites, benchmarks,
# benchmark harness (tests/perf) and CLI over every model in the checkout. See DEVELOPING.md.
# Usage: scripts/pgo-profile.sh [-keep DIR]   (-keep retains the per-workload profiles)
set -euo pipefail

cd "$(dirname "$0")/.."

keep=""
if [[ "${1:-}" == "-keep" ]]; then
  keep=$2
  mkdir -p "$keep"
fi
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

# profile NAME PKG [go test flags...]: run a package's tests or benchmarks under the CPU profiler.
profile() {
  local name=$1 pkg=$2
  shift 2
  echo "profiling $name"
  if ! go test "$pkg" -o "$work/$name.test" -cpuprofile "$work/$name.cpu" "$@" >"$work/$name.log" 2>&1; then
    echo "  $name failed; see $work/$name.log" >&2
    tail -n 20 "$work/$name.log" >&2
    exit 1
  fi
}

# Test-suite hot paths, including the corpus gates (corpus) and the stdlib gate (libs).
profile tests-parser  ./internal/syntax/parser  -count=1
profile tests-golden  ./tests/parser          -count=1
profile tests-resolve ./internal/semantic/resolve -count=1
profile tests-passes  ./internal/check/passes  -count=1
profile tests-runtime ./internal/exec/runtime -count=1
profile tests-model   ./internal/workspace/model   -count=1
profile tests-corpus  ./tests/corpus          -count=1
profile tests-libs    ./internal/workspace/libs    -count=1
profile tests-lsp     ./internal/lsp          -count=1

# Benchmarks: calc/instantiation/state machines (repl), the gRPC parse path, the harness.
profile bench-repl ./internal/repl      -run '^$' -bench . -benchtime 1s
profile bench-grpc ./internal/grpc      -run '^$' -bench . -benchtime 1s
profile bench-perf ./tests/perf -run '^$' \
  -bench 'Lex|Parse|IndexAdd|Analyze|WorkspaceEdit|FQNOf|LookupQualified|REPL|Lower|Execute|BatchConstraints|SameConstraint|GRPC|Connect' \
  -benchtime 1s

# The CLI: validate every example and corpus model (some are intentionally invalid,
# so that exit status is ignored), then load the stdlib and evaluate calc expressions.
echo "profiling cli"
go build -o "$work/sysml" ./cmd/sysml
mapfile -d '' models < <(find examples tests/testdata -type f \( -name '*.sysml' -o -name '*.kerml' \) -print0 | sort -z)
"$work/sysml" -validate -cpuprofile "$work/cli-validate.cpu" "${models[@]}" >"$work/cli-validate.log" 2>&1 || true
"$work/sysml" -cpuprofile "$work/cli-eval.cpu" -e '1 + 2 * 3' -e '(1..100)->collect{in x; x * x}' \
  -e 'RobotBehavior::Approach(3.0, 4.0)' examples/disposal-robot-demo/robot.sysml >"$work/cli-eval.log" 2>&1 ||
  { echo "  cli-eval failed; see $work/cli-eval.log" >&2; tail -n 20 "$work/cli-eval.log" >&2; exit 1; }

# Merge into one profile and install it beside each main package.
go tool pprof -proto "$work"/*.cpu >"$work/default.pgo" 2>/dev/null
for dir in cmd/sysml cmd/sysml-lsp cmd/sysml-grpc; do
  cp "$work/default.pgo" "$dir/default.pgo"
done
if [[ -n "$keep" ]]; then
  cp "$work"/*.cpu "$work"/*.log "$keep/"
fi
ls -l cmd/*/default.pgo
