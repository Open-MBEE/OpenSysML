- **`@Probability` weights state transitions.** The `Stochastic::Probability` notation that
  weights the branches out of a decision node now weights transitions too: of the transitions
  out of a state competing on one trigger (or the completion transitions), of all the branches
  out of a `choice` or `junction`, all carry a weight or none does, each weight lies in
  `[0, 1]`, and the group's weights must sum to one — checked at lowering for constants and at
  dispatch for expressions. A weighted pick is drawn once among the enabled transitions, after
  triggers, guards and innermost-wins have run, and is recorded in the witness, so `%replay`
  reproduces it, `explore` enumerates every weighted alternative, and the trace prints the
  drawn branch with its weight.
