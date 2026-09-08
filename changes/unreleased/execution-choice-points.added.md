- **The executors report every choice point: a pick among alternatives the library leaves
  unordered.** Several steppable tokens in one action step, several holding guards at a decision
  node, several enabled transitions out of one state for one event, and two tokens writing one
  feature in one step are each recorded as a `choice` trace line naming the alternatives and the
  one taken (`choice step 3: tokens 2@left, 3@right (unordered; took 3@right first)`), as an
  informational diagnostic on `ExecuteAction`, `ExecuteState` and `RunAnalysis` responses, and as
  one summary line after `%step`, `%continue` and `%advance` (`2 choice points; %trace on to see
  them`). What the executor does is unchanged — reverse token order, first holding guard, first
  declared transition — so every existing result is the same; a decision node now evaluates every
  guard rather than stopping at the first that holds, so a trace of one with several guards shows
  the later guards being evaluated. The innermost-transition-wins rule between a substate and the
  state enclosing it is spec-defined order and is not reported.
