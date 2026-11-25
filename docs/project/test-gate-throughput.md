# Test gate throughput on a shared host

The Go test gate is sharded so CI can give its slowest packages their own runner.
On one machine, running those shards across several worktrees at once defeats
that design: each process recompiles the same dependencies into its own build
cache, fills a separate temporary directory, and competes for RAM and CPU. The
result is not only slow but untrustworthy — a wall-clock deadline can expire on
a loaded host while the same test passes in isolation in a fraction of the
time, and a constrained temporary filesystem or the OOM killer can end a run
with no useful diagnostic.

This page records the three local accommodations and one investigation:

- a bounded, shard-aware [gate runner](#gate-runner),
- a [shared cache recipe](#one-warm-shared-cache) for several worktrees,
- an [opt-in wall-clock scale](#opt-in-deadline-scaling) that changes deadlines
  only, and
- the [exploration profiling investigation](#exploration-cost-investigation).

None of them changes what the gate asserts, which tests run, or what CI does.

## Gate runner

`go run -C tools ./cmd/gate-runner` runs every declared CI test shard with a
small, configurable number of shards in flight. It is the recommended way to
run the whole gate on a development machine.

```bash
# The merge-queue tier: every shard, race detector on all but the sweeps.
go run -C tools ./cmd/gate-runner

# The pull-request tier (faster: no race detector), four shards at a time.
go run -C tools ./cmd/gate-runner -tier quick -concurrency 4

# See the exact commands before running them; no tests are executed.
go run -C tools ./cmd/gate-runner -plan

# Only the named shards; the names are checked against the discovered list.
go run -C tools ./cmd/gate-runner -tier quick -shards smt,grpc
```

Flags:

| Flag | Default | Meaning |
|---|---|---|
| `-tier` | `queue` | `quick` (`make test-quick`), `queue` (`make test-queue`) or `full` (`make test-shard`) |
| `-concurrency` | `$OPENSYSML_GATE_CONCURRENCY` or `4` | maximum shards in flight; `1` reproduces a fully serial run |
| `-p` | `1` | package parallelism (`go test -p`) inside each shard; `0` leaves Go's default |
| `-shards` | all | comma- or space-separated subset of the discovered shards |
| `-v` | off | print passing shards' output as well as failing ones |
| `-list`, `-plan` | off | print discovery or the planned commands and exit |
| `-root` | walk up from the working directory | repository root |

The runner covers the Go test shards — the units the Makefile and the CI Go
matrix call shards. CI's standalone gates (the corpus, solver and differential
jobs) are not shards and stay as their own jobs; they are not silently merged
into this runner. The runner discovers shards with `scripts/race-shard.sh --all`,
the same list the Makefile and CI matrices are held to by `tests/hygiene/race_shards_test.go`.
There is no second shard list in the runner. `tests/hygiene/gate_runner_test.go`
enumerates every declared shard and checks that each tier plans exactly one
invocation for it, so a newly added shard cannot silently escape the local
gate. The runner's own bounded worker pool and failure aggregation are covered
by `tools/cmd/gate-runner/main_test.go`.

The runner generates the bundled-library snapshot once, before starting
workers, and sets `SKIP_STDLIB_SNAPSHOT=1` for each shard. The three `make`
targets take that as a guard on the snapshot prerequisite, and the variable
reaches the recursive `make` calls inside `test-quick` and `test-queue`, so
concurrent shards do not each remove and regenerate the embedded snapshot
source under the tree. After the run, each shard is reported in shard
order with its wall time; failing shards print their captured output. The
process exits non-zero when any shard fails, and it attempts every shard even
after one fails, matching CI's `fail-fast: false`.

Each shard still uses the Makefile's selectors, skips and timeouts. `-p 1` is a
second bound: with four shards and `-p 1`, at most four Go test packages link
and run at once. Raise `-p` only when the host has memory to spare.

## One warm shared cache

Go's build cache is content-addressed and safe for concurrent processes; each
process also creates its own uniquely named working directory under `GOTMPDIR`.
Several worktrees can therefore share one cache instead of recompiling the
same dependencies for every lane. Put both locations on the home filesystem,
not on a small tmpfs:

```bash
export GOCACHE="$HOME/.cache/go-build"
export GOTMPDIR="$HOME/.cache/opensysml/go-tmp"
export TMPDIR="$HOME/.cache/opensysml/tmp"
mkdir -p "$GOTMPDIR" "$TMPDIR"

# Warm the cache once, then run the bounded gate from each worktree.
GOMAXPROCS=2 make build-sysml
GOMAXPROCS=2 go run -C tools ./cmd/gate-runner -tier quick -concurrency 4
```

Keep `GOCACHE` and `GOTMPDIR` identical across the lanes: a per-worktree cache
is what turns N lanes into N full compilations. A `.cache` directory under the
home filesystem has room for the race-instrumented packages; a `/tmp` tmpfs
commonly does not, and its quota error surfaces as an opaque build failure.
`GOMAXPROCS` caps scheduler parallelism per process and `-concurrency` caps the
number of processes, so the product is the real CPU demand.

If a cache is ever suspect — for example after an interrupted build or a disk
full event — remove it and let the next run refill it:

```bash
go clean -cache
rm -rf "$GOTMPDIR"/*
```

**Trade-off with CI.** Each CI shard runs in its own job with its own restored
cache, so an entry written by one job cannot be observed by another and a
corrupt entry cannot spread across a tier. The local recipe trades that
isolation for a warm cache: all lanes trust the same host and build the same
tree and module pins, and Go verifies an action before reusing its result.
That is appropriate for worktrees on one machine, not for mutually
untrusted tenants. Local runs also share one checkout per worktree, so the
runner serializes the snapshot step that writes into the tree; CI jobs have
separate checkouts and do not need that step.

## Opt-in deadline scaling

A few tests assert that work finishes within a wall-clock deadline: parsing a
large batch of documents, analysing a wide view or an import cycle, rendering
a wide PDF table, and stopping a cancelled tool call. Those deadlines are
about detecting a hang or a regression, not about the host's spare CPU. When
several suites run at once they can expire on honest work.

Set `OPENSYSML_TEST_BUDGET_SCALE` to a positive number to stretch **those
deadlines only**:

```bash
OPENSYSML_TEST_BUDGET_SCALE=3 go run -C tools ./cmd/gate-runner -tier quick
```

Unset or empty, the scale is exactly `1` and every deadline has its declared
value. CI does not set the variable; `tests/hygiene/test_budget_test.go`
enforces that, and `tests/testutil/testbudget` holds the single reader of it.
An invalid value (non-numeric, non-positive, non-finite or greater than 1000)
fails at the point of use rather than being silently ignored.

The helper scales only the duration a test waits or compares against. It does
not change an expected diagnostic, an outcome, an exploration run or depth
budget, a step bound, a recorded figure, or the assertion that over-budget work
is a failure. `tests/testutil/testbudget/budget_test.go` proves the default is
unchanged, that a configured scale reaches a real deadline, and that an
over-budget run still fails at scale `1`. The runtime and sweep suites already
carry a fixed watchdog multiplier; it is left alone, so the opt-in is
additive there and applies to the unscaled deadlines elsewhere.

Reach for this only while interpreting a contended run. A failure that
persists at a larger scale is a real failure and must be investigated on its
own; do not raise the scale to make a deterministic assertion pass.

## Exploration cost investigation

The spacecraft case in `cmd/sysml` explores a race at t=79 under
`-schedule explore:runs=500,depth=1024` and replays each witness. It was the
longest single case observed under contention. There was no benchmark for that
path, so `tests/perf/spacecraft_explore_bench_test.go` now drives the same
`repl` surface in process, varying runs and depth and reporting allocations.
The benchmark asserts that each run budget is actually reached, so it measures
the exploration rather than an early exit.

Measured on one host with `GOMAXPROCS=2`, `-benchtime=1x`, `-benchmem`
(one iteration each; wall time will differ by machine):

| Budget | Time | Bytes/op | Allocs/op |
|---|---|---|---|
| `runs=8,depth=64` | 0.34 s | 361 MB | 2.93 M |
| `runs=64,depth=128` | 2.28 s | 2.71 GB | 21.98 M |
| `runs=300,depth=1024` | 10.27 s | 12.52 GB | 101.09 M |
| `runs=500,depth=1024` | 17.31 s | 20.79 GB | 167.80 M |

The cost is close to linear in **runs**, not depth: the 64-run case at depth
128 and the 300-run case at depth 1024 both spend about 35 ms per run. A
contended run therefore multiplies an already allocation-heavy workload; the
CLI case's several 500-run invocations are the dominant cost.

A CPU and allocation profile of the 500/1024 case
(`-cpuprofile`/`-memprofile` on the benchmark) shows where that goes:

- Allocation profile: 21.2% of bytes allocated flow through
  `futureFootprints.machineParts`, 16.1% through `unionFootprints` (1.88 GB of
  that is the function's own `slices.Clone` calls on `Reads`, `Writes`, `Sends`,
  `Accepts`, `Control` and `Flows`), 9.3% through `addFootprint`, and 7% through
  the generic `sized` allocator; `slices.Clone` of `lower.Place` slices alone is
  4.8%. The concrete chain is
  `exploreQueue.explore → freshInvocation.run → Context.Advance → stepDue →
  contended (48.7% of bytes) / StateExecutor.runCounting (43.4%) →
  stepDoAction → futureFootprints.gate/machineParts`.
- CPU profile: about 26% of samples are in `runtime.mallocgc`, 18% in
  `runtime.growslice`, 13.6% in `runtime.scanSpan` and 11.9% in
  `runtime.tryDeferToSpanScan`. The largest named application frames are
  `holdsIdentityFrom` (5.0%) and `addFootprint` (6.6%). In other words, most
  CPU time is the garbage collector and slice growth caused by the allocation
  churn above, not the semantic decision itself.

This is an investigation only. The repo already has a single-pass
`sumFootprints` helper beside the repeated `unionFootprints` calls, which
suggests a direction for a future allocation reduction, but nothing here
changes the executor, the test's budgets or its witness assertions.
