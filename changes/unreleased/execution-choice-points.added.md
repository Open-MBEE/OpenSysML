- **The executors report every choice point: a pick among alternatives the library leaves
  unordered.** Several steppable tokens in one action step, several holding guards at a decision
  node, several enabled transitions out of one state for one event, and two tokens writing one
  feature in one step are each recorded as a `choice` trace line naming the alternatives and the
  one taken (`choice step 3: tokens 2@left, 3@right (unordered; took 3@right first)`), as an
  informational diagnostic on `ExecuteAction`, `ExecuteState` and `RunAnalysis` responses, and as
  one summary line after `%step`, `%continue` and `%advance` (`2 choice points; %trace on to see
  them`). What the executor does is unchanged — reverse token order, first holding guard, first
  declared transition — so every existing result and trace is the same: the guards and
  transitions after the first that holds are read in a preview that is undone, and one that
  cannot be evaluated there is not an alternative and not an error but an informational
  `guard-unevaluable` diagnostic, an `unevaluable guard` trace line and a count in the summary
  (`1 guard not evaluable`). Writes to the performing part and through a feature chain count as
  the object's, so two chains reaching one object in one step are one conflict. The
  innermost-transition-wins rule between a substate and the state enclosing it is spec-defined
  order and is not reported.
