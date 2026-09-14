- **The SMT model-checking note reads timed accepts without a horizon, and the encoding numbers
  flows by frame.** `docs/internals/design/smt-model-checking.md` now says, in one place, how
  `now` is to advance under `-engine smt`: only when no token is enabled, to the earliest due
  time, with `k` moves alone bounding the run and `-advance` refused as `check` refuses it — the
  library's `Clocks` and `Occurrences` fix when a timed accept becomes enabled and name no
  horizon, so none is added; the coverage row that said timed accepts needed `-advance` is
  corrected. In `internal/core/smt`, `Flow` numbers the root graph and every flow a node states
  of its own as frames, each with its own node range, labels and slot count, and records the
  `send` and `accept` sites it meets; `State` declares parked, due, clock and bus variables only
  for a flow that has accepts, timed accepts or sends, and lists its variables as a named vector.
  No transition reads them yet: `send`, `accept`, `accept after`/`accept at` and a node stating
  a flow of its own are refused before any query exactly as before, and no `-engine smt` output
  changes.
