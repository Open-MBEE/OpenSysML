# Test gate throughput on a shared host

The Go test gate is sharded so CI can give its slowest packages their own runner.
On one machine, running those shards across several worktrees at once defeats
that design: each process recompiles the same dependencies into its own build
cache, fills a separate temporary directory, and competes for RAM and CPU. The
result is not only slow but untrustworthy — a wall-clock deadline can expire on
a loaded host while the same test passes in isolation in a fraction of the
time, and a constrained temporary filesystem or the OOM killer can end a run
with no useful diagnostic.

This page records three local accommodations, one opt-in investigation and one
measurement script:

- a bounded, shard-aware [gate runner](#gate-runner),
- a [shared cache recipe](#one-warm-shared-cache) for several worktrees,
- an [opt-in wall-clock scale](#opt-in-deadline-scaling) that changes deadlines
  only,
- the [exploration profiling investigation](#exploration-cost-investigation), and
- an [A/B timing script](#measured-gate-timing) over two shards.

None of them changes what the gate asserts, which tests run, or what CI does.

## Gate runner

`go run -C tools ./cmd/gate-runner` runs every declared CI test shard with a
small number of shards in flight. It derives that number, each shard's
`GOMAXPROCS` and the retained log size from the host, and it is the recommended
way to run the whole gate on a development machine.

```bash
# The merge-queue tier: every shard, race detector on all but the sweeps.
go run -C tools ./cmd/gate-runner

# The pull-request tier (faster: no race detector).
go run -C tools ./cmd/gate-runner -tier quick

# See the exact commands before running them; no tests are executed.
go run -C tools ./cmd/gate-runner -plan

# Only the named shards; the names are checked against the discovered list.
go run -C tools ./cmd/gate-runner -tier quick -shards smt,grpc
```

Flags:

| Flag | Default | Meaning |
|---|---|---|
| `-tier` | `queue` | `quick` (`make test-quick`), `queue` (`make test-queue`) or `full` (`make test-shard`) |
| `-concurrency` | derived from CPUs and RAM | maximum shards in flight; override with `$OPENSYSML_GATE_CONCURRENCY` |
| `-p` | `1`, or an explicit `GOFLAGS` `-p` | package parallelism (`go test -p`); the per-process CPU allowance is divided by it so the product stays within the shard budget |
| `-gomaxprocs` | derived from CPUs | `GOMAXPROCS` per package test process, after dividing by `-p`; an explicit caller `GOMAXPROCS` wins, override with `$OPENSYSML_GATE_GOMAXPROCS` |
| `-log-tail` | `64K` | bytes of a failing shard's log to keep; accepts a `K`, `M` or `G` suffix, override with `$OPENSYSML_GATE_LOG_TAIL` |
| `-shards` | all | comma- or space-separated subset of the discovered shards |
| `-v` | off | stream every shard's output as it arrives rather than buffering it |
| `-list`, `-plan` | off | print discovery or the planned commands and exit |
| `-root` | walk up from the working directory | repository root |

**Derivation rule.** The runner first reads the effective CPU limit and
available memory from the process's own cgroup *and every ancestor*: a quota
or memory limit on a parent in a systemd slice bounds the process just as its
own cgroup does, and the walk takes the tightest CPU quota and the smallest
finite memory headroom. Where no controller limit is found it falls back to
`runtime.NumCPU` and `MemAvailable` (itself a cap on a finite cgroup headroom).
The default shard count is the smaller of the CPU count and `available memory /
2 GiB`, at least one. The `GOMAXPROCS` per shard is then `CPUs / workers`, where
`workers` is the number of shards that will actually be in flight after the
`-shards` subset and `-concurrency` cap are applied — not the default shard
count. A 16-CPU, 16-GiB machine gets eight shards with `GOMAXPROCS=2`; asking
that machine for four shards in flight gives each `GOMAXPROCS=4`, and a
two-shard subset gives each `GOMAXPROCS=8`. Explicit flags and environment
variables override any derived value, so CI and developers keep control; the
runner never imposes this host's settings on another one. By default it passes
`-p=1`, so one package test binary runs at a time with the whole shard
allowance. An explicit `-p` or one already in `GOFLAGS` is preserved and divides
that allowance among the concurrent package processes, so their product stays
within the budget. `GOFLAGS` is read through `go env`, so a value saved with
`go env -w` but not exported is kept instead of being replaced by `-p` alone.
It is tokenized the way Go does — quoted arguments stay whole, both `-p` and
`--p` are recognized, and the last setting wins — so an embedded `-p=` inside
another flag's quoted value is never mistaken for a package limit.

The runner covers the Go test shards — the units the Makefile and the CI Go
matrix call shards. CI's standalone gates (the corpus, solver and differential
jobs) are not shards and stay as their own jobs. The runner discovers shards
with `scripts/race-shard.sh --all`, the same list the Makefile and CI matrices
are held to by `tests/hygiene/race_shards_test.go`. There is no second shard
list in the runner. `tests/hygiene/gate_runner_test.go` enumerates every
declared shard and checks that each tier plans exactly one invocation for it.

The runner generates the bundled-library snapshot once, before starting
workers, and sets `SKIP_STDLIB_SNAPSHOT=1` for each shard. The three `make`
targets take that as a guard on the snapshot prerequisite, and the variable
reaches the recursive `make` calls inside `test-quick` and `test-queue`, so
concurrent shards do not each remove and regenerate the embedded snapshot
source under the tree. Each shard is reported in shard order with its wall
time; failing shards print their captured output. A shard that exits zero but
prints a `GATE NOT RUN` banner — the corpus it was asked to check is absent —
is reported as `WARN`, not `PASS`, and the closing line counts the incomplete
shards. The banners are counted from the whole stream, so they survive a
successful shard and `-v` output even though the rolling log tail is kept only
for failures. This is a status, not a new failure mode: a required corpus
still fails its shard and exits non-zero through the existing require
variables. The process exits non-zero when any shard fails, and it attempts
every shard even after one fails, matching CI's `fail-fast: false`.

The tools module runs with `tools` and `rest` in CI's separate jobs. When a
local plan contains both, the runner passes `SKIP_TOOLS_TESTS=1` to `rest` so
the module and its snapshot generator test run once; a plan with only `rest`
keeps that coverage. The owning shard attempts the WebAssembly package tests
and the tools module tests independently and fails if either fails, so a
WebAssembly failure no longer suppresses the module's tests. The generator's
default-path test writes its snapshot into a fixture checkout, so its own
execution cannot truncate a snapshot another shard is embedding.

A shard's output is never held in full: it is streamed to the terminal with
`-v`, and otherwise only the last `-log-tail` bytes are kept for a failing
shard's diagnostic. The runner handles `SIGINT` and `SIGTERM` by putting each
shard in its own session and signalling every process group still in it: a
descendant that starts a group of its own, as an analysis engine stand-in does,
is found through `/proc` and stopped too. Unit tests cover large-log
truncation, verbose streaming, bounded concurrency, the snapshot-once guard,
descendant cleanup (including an own-group child), skip-banner status and the
cgroup hierarchy with fixture trees for a limit at the process cgroup and one
inherited from a parent.

## One warm shared cache

Go's build cache is content-addressed and safe for concurrent processes; each
process also creates its own uniquely named working directory under `GOTMPDIR`.
Several worktrees can therefore share one cache instead of recompiling the
same dependencies for every lane. These variables are **options for local
work**, not requirements: leave them at their defaults if you are running one
worktree. When several lanes compete, put the caches on the home filesystem
rather than a small tmpfs:

```bash
# Optional, for concurrent worktrees only.
: "${GOCACHE:=$HOME/.cache/go-build}"
: "${GOTMPDIR:=$HOME/.cache/opensysml/go-tmp}"
: "${TMPDIR:=$HOME/.cache/opensysml/tmp}"
mkdir -p "$GOTMPDIR" "$TMPDIR"
export GOCACHE GOTMPDIR TMPDIR

# Warm the cache once, then run the bounded gate from each worktree.
go build ./...
go run -C tools ./cmd/gate-runner -tier quick
```

Keep `GOCACHE` and `GOTMPDIR` identical across the lanes that share a host: a
per-worktree cache is what turns N lanes into N full compilations. A `.cache`
directory under the home filesystem has room for the race-instrumented
packages; a `/tmp` tmpfs commonly does not, and its quota error surfaces as an
opaque build failure. If a cache is ever suspect, remove it and let the next
run refill it:

```bash
go clean -cache
# Only remove from an explicit, approved root: an unset GOTMPDIR must never
# turn the glob into a deletion against the filesystem root.
: "${HOME:?HOME must be set to resolve the approved cache root}"
: "${GOTMPDIR:?set GOTMPDIR to the shared cache directory before removing it}"
cache_root="$HOME/.cache/opensysml"
case "$GOTMPDIR" in
  "$cache_root"/*) ;;
  *) printf 'refusing to remove outside %s\n' "$cache_root" >&2; exit 1 ;;
esac
# Resolve both paths before deleting: a textual prefix alone accepts parent
# components and symlinks whose targets lie outside the approved root.
approved=$(realpath -m -- "$cache_root") || exit 1
resolved=$(realpath -e -- "$GOTMPDIR") || { printf 'GOTMPDIR does not resolve\n' >&2; exit 1; }
case "$resolved" in
  "$approved"/*) ;;
  *) printf 'refusing to remove outside %s\n' "$approved" >&2; exit 1 ;;
esac
# Reject symlinks on every component below HOME before removing anything.
walked=$HOME
IFS='/' read -r -a parts <<< "${GOTMPDIR#"$HOME"/}"
for part in "${parts[@]}"; do
  case "$part" in
    ''|.) continue ;;
    ..) printf 'refusing a parent component in %s\n' "$GOTMPDIR" >&2; exit 1 ;;
  esac
  walked+="/$part"
  if [[ -L "$walked" ]]; then
    printf 'refusing a symlinked cache path: %s\n' "$walked" >&2
    exit 1
  fi
done
rm -rf -- "$resolved"/*
```

Both paths are resolved and symlinks on every component below `HOME` are
rejected before deletion, so neither a `..` component nor a link to another
directory can widen the approved root.

**Trade-off with CI.** Each CI shard runs in its own job with its own restored
cache, so an entry written by one job cannot be observed by another and a
corrupt entry cannot spread across a tier. CI keeps those isolated caches and
does not use the variables above; the local recipe trades that isolation for a
warm cache, trusting every lane on one host to build the same tree and module
pins. Go verifies an action before reusing its result. Local runs also share
one checkout per worktree, so the runner guards the snapshot step that writes
into the tree; CI jobs have separate checkouts and do not need that step.

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
value. No CI file sets the variable; `tests/hygiene/test_budget_test.go`
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

## Measured gate timing

`scripts/gate-timing.sh` is an opt-in measurement, not a CI gate. It warms one
shared cache and times the serial and bounded runners over the same two shards:

```bash
GOMAXPROCS=2 scripts/gate-timing.sh
```

The commands it times are
`/usr/bin/time -f '%e s %M KB' make test-quick SHARDS="grpc rest"` and
`/usr/bin/time -f '%e s %M KB' go run -C tools ./cmd/gate-runner -tier quick -shards grpc,rest -concurrency 2`.
Both use `GOMAXPROCS=2` and the home-filesystem `GOCACHE`, `GOTMPDIR` and
`TMPDIR`. Both also export `GOFLAGS=-count=1`, which disables Go's successful
test-result cache: neither arm can warm a cached replay for the other. The
shared build cache stays warm for both (it only affects compilation), so the
figures measure test execution and runner overhead, **not cache hits**. Before
timing, the script regenerates the embedded-library snapshot and compiles the
exact test sets of both shards through `make test-quick` with `-run '^$'`, so no
test runs during warmup. That keeps `-pgo=off`, each shard's `-run`/`-skip`
selection and the timeout identical to the measured runs, and the snapshot is
present before anything compiles. The recorded figures below predate those
changes; re-run the script for a directly comparable current measurement.
Replace the shards in the script to measure another pair.

`GATE_TIMING_TMP` may name a relative directory under `$HOME/.cache/opensysml`,
where the script puts its temporary directories. The entire value is checked:
an absolute path, a `..` component, an unsupported character (including any
control character) or a symlinked path component is rejected before `mkdir`
runs, the directory is rebuilt from the validated components, and its resolved
path must still be inside the approved root. With the variable unset the script
uses `$HOME/.cache/opensysml/gate-timing`.

Recorded on **one host** (16 CPUs, 31264 MiB RAM, load average 2.40, Go 1.27.4,
`GOMAXPROCS=2`, warm home-filesystem cache; both runs exited zero):

| Invocation | Wall time | Peak RSS |
|---|---|---|
| `make test-quick SHARDS="grpc rest"` | 418.67 s | 6184384 KB |
| `go run -C tools ./cmd/gate-runner -tier quick -shards grpc,rest -concurrency 2` | 721.92 s | 8868652 KB |

The bounded runner is **not** a speedup on this two-shard, single-lane
comparison. Both sides ran with the caller's exported `GOMAXPROCS=2`: the
serial invocation carries it into each of its two shards in turn, and the
runner propagates it to both of its shard children because an explicit
`GOMAXPROCS` wins over the runner's derived value. The runner's extra process
and concurrent execution cost is the observable difference here. Its value is
bounding total demand when several worktrees share a host, not making one
quiet run faster; read these numbers as one host's measurement, not a gate.

The multi-worktree benefit is an **expected bound from the design**, not a
measured result: the cgroup-aware CPU limit times the shard count stays within
the host's CPU budget, and the shared cache removes duplicate compilation. A
maintainer should measure it by running two worktrees' quick-tier gates
concurrently against one warm cache and comparing wall time and peak RSS with
a single-lane run; that two-worktree measurement is not part of this record.

## Exploration cost investigation

The spacecraft case in `cmd/sysml` explores a race at t=79 under
`-schedule explore:runs=500,depth=1024` and replays each witness. It was the
longest single case observed under contention. `tests/perf/spacecraft_explore_bench_test.go`
drives the same `repl` surface in process, varying runs and depth and reporting
allocations. The benchmark asserts that each run budget is actually reached,
so it measures the exploration rather than an early exit.

Measured on **one host** (AMD Ryzen 7 7840HS, `GOMAXPROCS=2`, `-benchtime=1x`,
`-benchmem`, one iteration each). These are observations, not gates:

| Budget | Time | Bytes/op | Allocs/op |
|---|---|---|---|
| `runs=8,depth=64` | 0.34 s | 361 MB | 2.93 M |
| `runs=64,depth=128` | 2.28 s | 2.71 GB | 21.98 M |
| `runs=300,depth=1024` | 10.27 s | 12.52 GB | 101.09 M |
| `runs=500,depth=1024` | 17.31 s | 20.79 GB | 167.80 M |

An independent reproduction of the small case measured 0.383 s, 361 MB and
2.93 M allocations, matching the 8-run row. The cost is close to linear in
**runs**, not depth: the 64-run case at depth 128 and the 300-run case at depth
1024 both spend about 35 ms per run. A contended run therefore multiplies an
already allocation-heavy workload; the CLI case's several 500-run invocations
are the dominant cost.

A CPU and allocation profile of the 500/1024 case (`-cpuprofile`/`-memprofile`
on the benchmark) shows where that goes:

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
  `holdsIdentityFrom` (5.0%) and `addFootprint` (6.6%). Most CPU time is the
  garbage collector and slice growth caused by the allocation churn, not the
  semantic decision itself.

This is an investigation only. The repo already has a single-pass
`sumFootprints` helper beside the repeated `unionFootprints` calls, which
suggests a direction for a future allocation reduction, but nothing here
changes the executor, the test's budgets or its witness assertions.
