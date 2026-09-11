- **The self-model's question and exploration flows follow the dispatcher and the explorer.**
  `AnswerQuestion` gives `all` a branch of its own that consults the engines declaring the
  question's kind in name order, as `Registry.Answer` does, where before it shared `auto`'s
  authority ranking; the three selections join before the coverage check, and the plan advances
  only while candidates remain — the engines declaring the kind (`candidates`, one per kind in the
  default registry, where every engine declares a kind of its own), not the whole registry, with
  every engine consulted landing in the plan as a step (`steps`). `ExploreOutcomes` charges a run to
  the budget for every linearization it commits and works the queue it drains — the empty prefix to
  start, then the prefixes each run leaves unexplored (`unexplored`), taken up once so that a run
  below every choice leaves none — so a finite choice tree drains the queue before the 1024-run
  bound and proves, a tree the bound cuts observes, and neither flow depends on a fixed decision
  any more. Both flows now run under `go test ./examples/`, through several candidate counts,
  selections and choice trees. The pilot differential baseline is re-recorded from one validator
  run over the current `examples` tree — 369 files, 338 fully agreeing, 671 pilot-only, 707 pilot
  diagnostics — so its per-file rows and provenance digest measure the same inputs again; the
  record and the generated figures follow.
