- **The self-model's question and exploration flows follow the dispatcher and the explorer.**
  `AnswerQuestion` gives `all` a branch of its own that consults the registry's engines in name
  order, as `Registry.Answer` does, where before it shared `auto`'s authority ranking; the three
  selections join before the coverage check and the plan advances only while engines remain.
  `ExploreOutcomes` charges a run to the budget for every linearization it commits and keeps the
  queue it drains — the empty prefix to start, one more prefix per choice a run meets — so the
  1024-run bound and the drained queue both end exploration, where before neither the budget nor
  the queue moved and the flow depended on a fixed decision. The pilot differential baseline is
  re-recorded from one validator run over the current `examples` tree — 369 files, 338 fully
  agreeing, 671 pilot-only, 707 pilot diagnostics — so its per-file rows and provenance digest
  measure the same inputs again; the record and the generated figures follow.
