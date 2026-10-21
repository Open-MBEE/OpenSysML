# Performance: release 0.11.0 against release 0.10.1

> **Work in progress.** The six-round measurement is still running; the
> figures below are from the first round only and every regression they
> show is being attributed. The record is rewritten in full when the run
> completes.

The release-gate measurement of the 0.11.0 candidate — the head of
`develop` at `b60a18c2087f21bf58111da322435b7d701fc86d` (2026-10-21), from
which `release/0.11.0` is to be cut — measured against the `v0.10.1` tag
on `main` (`1c45b6e17`). It follows the method of the
[0.10.1 against 0.10.0](performance-release-0.10.1-vs-0.10.0.md) record
and of the [0.10.0 against 0.9.2](archive/performance-release-0.10.0-vs-0.9.2.md)
record before it: every benchmark on both revisions, `benchstat` over six
runs, the rows the machine leaves in doubt re-run interleaved, and
whole-binary timings on generated and real models. See
[performance](../internals/performance.md) for what the figures mean and
where the remaining cost is.

## Method

Two detached worktrees, one per revision, each built with `make build`
(`go1.25.11 linux/amd64`). Every benchmark package is run with
`-run '^$' -bench . -benchmem -count 1`, base then candidate, six rounds
in a row, so the two revisions see the same machine state round by round;
each round's output is an append-only file and `benchstat` compares the
six files per revision. `tests/stressmodel` runs the same way with
`ConvertAPIJSON/satellites=512` skipped, as the previous records did.

The machine is an 8-vCPU `Intel(R) Xeon(R) Processor` (GenuineIntel, one
thread per core, 320 MiB L3, 31 GiB), `GOMAXPROCS=8`. It is not the host
of the 0.10.1 record, so sec/op is comparable only between the two columns
of this record, never with the earlier records; allocations and bytes are
exact and comparable across hosts.

## First-round findings

Allocation counts are exact, so these rows are regressions regardless of
how the remaining rounds place their times:

- `internal/frontend/lsp`: `ReferencesCold` 44.6k → 6.28M allocs/op
  (9.8 MiB → 1 010 MiB, 45.9 ms → 1 206 ms); `RenameWarm` 14.6k → 12.4M
  allocs/op (1.4 MiB → 1 668 MiB, 21.0 ms → 2 374 ms). `ReferencesWarm`,
  `WorkspaceUpdate`, `FormatEdits` and `SpanToRangeLarge` are unchanged
  to the allocation.
- `internal/exec/runtime`: the arithmetic loop rows carry two more
  allocations per iteration (`IntegerArithmeticLoop` 9 072 → 11 073,
  280 → 483 KiB; `RationalDecimalLoop` and `RealDecimalLoop` likewise;
  `IntegerArithmeticNestedLoop` 26.08k → 30.08k), `NewRootFrame` 36 → 55,
  `DerivedReadWriteRead` 65 → 71, `AssignmentLoopStep` 7 328 → 7 529.
- `tests/perf`: `ImportValues/*` +34–44% allocations, `FeaturesOf`
  4.130M → 5.416M, `Analyze/synthetic` 4.382M → 4.832M, `BatchSatisfy`
  4.301M → 5.278M, `Instantiate` 3.081M → 3.746M, `ExecuteAction`
  245 → 297, `ActionLoop/for1000` 4 231 → 6 264, `StateLoop/count5000`
  1.176M → 1.371M. `Parse`, `Lex`, `IndexAddExpand`, `WorkspaceEdit`,
  `FQNOf`, `LookupQualified`, `REPLEvalExpr`, `LowerActionGraph`,
  `LowerChain` and `GRPCEvaluate` are at parity.
- `internal/frontend/repl`: `Diagnostics` +23–29% allocations,
  `LoadModel` +5–10%, `Instantiate` 56 → 59.
- `internal/workspace/libs`, `internal/workspace/model`,
  `internal/translate/migrate` and `internal/frontend/grpc` are at parity
  to the allocation.

Each row is being bisected over the pull-request merges between the two
revisions; the fixes, where a row is not the inherent cost of a
correctness change, go in their own pull requests and are listed here.
