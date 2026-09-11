- **The self-model's question and exploration flows follow the dispatcher and the explorer.**
  `AnswerQuestion` gives `all` a branch of its own that consults the engines declaring the
  question's kind in name order, as `Registry.Answer` does, where before it shared `auto`'s
  authority ranking; the three selections join before the coverage check, and the plan advances
  only while candidates remain — the engines declaring the kind (`candidates`, one per kind in the
  default registry, where every engine declares a kind of its own), not the whole registry, with
  every engine consulted — answering, refusing or faulting — landing in the plan as a step
  (`steps`), and a refusal (`refusing`, the candidates asked first that do not cover the
  question) moving on to the next candidate whatever the selection, as `Registry.answer` does.
  `ExploreOutcomes` charges a run to the budget for every linearization it commits and works the
  queue it drains — the empty prefix to start, then the prefixes each run leaves unexplored: the
  choice tree is modelled as a chain of choices (`choicesAhead`) of so many alternatives each
  (`alternatives`), the next below one alternative of the one above (`below`, the first by
  default), and the prefix in hand as the choice it ends at and the alternative it takes there
  (`choice`, `taken`); a run takes the first alternative of every choice it meets on its way down
  and leaves the second of each, and the next alternative of the choice its prefix ended at — one
  prefix per choice it owns, as `exploreRun.unexplored` does, not every alternative at once — and
  the next run takes the prefix queued deepest, so two binary choices met by the first run take
  three runs, not four, and a later run advances a four-way choice to its third alternative rather
  than finding three prefixes queued; what a run leaves goes on the queue only up to the runs left,
  the plan never outgrowing the run budget, the rest dropped and the runs bound marked hit
  (`runsHit`), as `exploreQueue.insert` does; a choice met beyond the depth bound (`depth`, 64 by
  default) takes its first alternative and marks that bound hit — so the queue always drains, a
  finite choice tree within the bounds proves, a tree either bound cut observes, and neither flow
  depends on a fixed decision any more. Both flows now run under
  `go test ./examples/`, through several candidate counts, selections, faults and choice trees.
  The pilot differential baseline is re-recorded from one validator run over the current
  `examples` tree — 370 files, 337 fully agreeing, 671 pilot-only, 707 pilot diagnostics — so its
  per-file rows and provenance digest measure the same inputs again; the record and the generated
  figures follow.
