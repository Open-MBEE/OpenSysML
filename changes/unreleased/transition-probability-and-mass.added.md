- **`@Probability` weights state transitions.** The `Stochastic::Probability` notation that
  weights the branches out of a decision node now weights transitions too: of the transitions
  out of a state competing on one trigger (or the completion transitions), of all the branches
  out of a `choice` or `junction`, all carry a weight or none does, each weight lies in
  `[0, 1]`, and the group's weights must sum to one — checked at lowering for constants and at
  dispatch for expressions. A weighted pick is drawn once among the enabled transitions, after
  triggers, guards and innermost-wins have run, and is recorded in the witness, so `%replay`
  reproduces it, `explore` enumerates every weighted alternative, and the trace prints the
  drawn branch with its weight.
- **`explore` and `check` report probabilities.** The explore outcome table gains a
  `probability` column — the product of the shares each linearization's picks resolved with (a
  weighted pick its stated weight's share, an unweighted choice the uniform `1/n` a seed takes
  each alternative with), summed over the runs reaching each outcome — and `check` reports each
  violation's probability mass the same way, `(probability 0.3)` on its line and `mass` in the
  JSON report. Both are the model's own probabilities where every choice point is weighted, a
  uniform assumption otherwise; an incomplete exploration prefixes them `≥` and a check that
  hit a bound, revisited a state or left a move out marks them lower bounds
  (`probabilitiesLowerBound` / `massLowerBound` in the JSON, `Outcome.probability` and
  `ExplorationStatus.probabilities_lower_bound` on the wire).
