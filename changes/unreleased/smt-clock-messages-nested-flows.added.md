- **`-engine smt` covers the clock, messages and nested flows.** The bounded transition relation
  now encodes `send` and `accept` within the checked behavior over a bus of `M` slots (`M` the
  `Send` statements reachable within `k` moves, reported in `Bounds` as `bus` for a behavior
  that sends), `accept after` and `accept at` on one clock (`now` advances only when no token is
  enabled, to the earliest due time; no horizon, `k` moves alone bound the run, and `-advance`
  stays refused for `smt`), and a node stating a flow of its own as nested token slots that
  complete the node when the nested flow reaches its finals. Two waiters for one message and
  tokens due at one instant are the interpreter's own `ChoiceTokenOrder` and `ChoiceDueOrder`
  choices, spelled in the witness as `explore` writes them so a `replay:` file from `smt` walks
  in the debugger. The referee compares the corpus's `accept`, `send`, clock and nested-flow
  action cases against `explore`.
