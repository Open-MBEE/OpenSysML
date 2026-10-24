# Performance census, September 2026

A measurement of where the toolchain stands after the past week of changes:
week-over-week benchmark movement, and whole-binary scaling against the
figures recorded in `docs/internals/performance.md`.

All figures were taken on one machine — `Intel Xeon Platinum 8559C`, 8 CPUs,
Go 1.25, Linux — the same processor family the figures in
`docs/internals/performance.md` were taken on, so the two sets compare
directly. Treat absolute numbers as ratios elsewhere.

## Week over week: `main` against `main` seven days earlier

`go test ./internal/repl -run '^$' -bench . -benchmem -count 6`, compared with
`benchstat`. The baseline is the merge at the top of `main` seven days before
the measurement (`01b2b86d`, 2026-08-25); the measured revision is `3e5bacb7`.
The benchmark's synthetic model changed between the two revisions — the
notation it used was removed from the grammar in the interval — so the two
sides load equivalent but not byte-identical models; the run benchmarks
(state machine, calc, instantiate) also execute under a runtime whose
completion semantics changed in the same interval.

What moved, with `benchstat` confidence (p ≤ 0.004 unless marked):

| figure | 2026-08-25 | 2026-09-01 | movement |
| ------ | ---------- | ---------- | -------- |
| load wall, 250/1000/4000 elements | 16.8 / 66.8 / 277 ms | 19.4 / 77.9 / 291 ms | +15% / +17% / +5% |
| load live heap, 250/1000/4000 | 2.49 / 9.77 / 39.0 MiB | 2.15 / 8.26 / 32.8 MiB | −14% / −16% / −16% |
| load allocations, 250/1000/4000 | 124k / 486k / 1.94M | 88k / 341k / 1.35M | −29% / −30% / −30% |
| empty-session load wall | 162 µs | 1024 µs | +532% |
| empty-session bytes allocated | 111 KiB | 695 KiB | +525% |
| state-machine start, 4000 elements | 158 µs | 221 µs | +40% |
| state-machine start bytes/allocs | 5.3 KiB / 81 | 9.0 KiB / 120 | +71% / +48% |
| diagnostics benchmarks | — | — | unchanged |

Loading holds noticeably less and allocates far fewer objects than a week ago,
at a modest wall-time cost at small sizes that fades by 4 000 elements. Two
movements deserve attention:

- **The empty-session floor grew six-fold.** A heap profile attributes the
  growth to `semantics.(*Model).collectAboutAnnotations`: building the
  `about`-metadata index walks every document in the index — the bundled
  standard library included — once per session, and the visited-symbol set for
  that walk is most of the new allocation. The cost is fixed per session
  (~0.9 ms, ~580 KiB of garbage; live heap is unchanged at 32 KiB), so a large
  model amortizes it, but a process serving many short sessions pays it each
  time. Restricting the walk to workspace documents unless the library itself
  declares `about` usages — a fact computable once when the library index is
  built — would restore the old floor.
- **Starting a state machine costs 40% more wall and 71% more memory** than a
  week ago at 4 000 elements. The interval removed the final-state marker in
  favor of completing on a transition to `done` and rewrote the benchmark's
  machine into standard notation, so the two sides execute different models;
  the movement is explained rather than adjudicated here, and the new figures
  are the baseline to watch.

## Whole-binary scaling

`sysml -validate` over generated compliant models in standard notation that
report nothing, three runs each. The documented figures are the table in
`docs/internals/performance.md`.

| elements | documented | measured, this census | peak RSS documented / measured |
| -------- | ---------- | --------------------- | ------------------------------ |
| 3 000    | 0.30 s     | 0.33 s                | 117 / ~120 MiB                 |
| 6 000    | 0.49 s     | 0.55–0.61 s           | 159 / ~180 MiB                 |
| 12 000   | 0.92 s     | 1.04 s                | 272 / ~285 MiB                 |

Both time and memory remain linear in the model. The measured points sit
5–12% above the documented ones and the seven-day-old binary measures the
same within noise, so the drift against the documented table predates this
week and is within machine-to-machine variation.

## Batch validation of many files is quadratic

The OMG training corpus — 100 compliant files, 572 KiB — validated as one
batch is the many-small-files shape rather than the one-large-model shape,
and it costs 6.7 s at 168 MiB peak, while its two halves cost 0.6 s and
3.4 s separately — the whole costs far more than its parts. Per-file, every
file is fast alone (the slowest is 0.33 s). A CPU profile of the batch shows
where the time goes: the CLI submits files to the session one at a time, and
every submission reindexes the workspace
(`model.(*Workspace).setOpenBuffer` → `reindexLocked`, 52% of the profile),
re-running `symbols.(*Index).ExpandWildcardImports` over every document
loaded so far — so a batch of *n* files expands wildcard imports *n* times
over a growing buffer, and the total is quadratic in the file count.

This is not a regression — the seven-day-old binary measures the same — and
the cost is in the submission loop rather than in analysis itself.
Submitting a `-validate` batch as one indexing unit, or expanding wildcard
imports incrementally for documents the new submission cannot affect, would
make the corpus cost what its parts cost.

Since this census, the CLI loads a `-validate` batch as one submission
(`Session.LoadFilesSummary`), indexed and analyzed once, with each file still
summarized on its own and every diagnostic reported as before. The corpus now
validates in 0.30 s (six runs, ±2%), against 0.12 s for its first 25 files and
0.13 s for its first 50 — the constant floor plus a term linear in the input.

## Assessment

Single-model validation remains linear in both time and memory through
12 000 elements, loading holds less live heap and allocates a third fewer
objects than a week ago, and the two costs worth acting on are internal and
localized: the per-session `about`-metadata walk that grew the empty-session
floor, and the quadratic submission loop in many-file batch validation.
