- **The `explore` scheduling policy runs every linearization of a behavior and tables its
  distinct outcomes.** `sysml -schedule explore[:runs=N,depth=D]` with `-action`, `-state`,
  `-analysis` or `-calc` runs the behavior once, recording the alternative taken at each choice
  point, then replays it from the start on a fresh executor of the same loaded model — no object,
  message, clock, calc memo or note of one run is seen by the next — following the recorded prefix
  and taking the next untried alternative at the frontier, depth-first, until every choice
  sequence is spent or a budget is hit. Runs that agree on the observables the conformance
  harness compares (an action's outputs; a state machine's final state, states visited and values;
  an analysis case's outputs and verdicts) are one outcome; the report is one sorted row per
  distinct outcome with the number of linearizations reaching it and the choice sequence of one
  witness, then `complete (N runs)` or `incomplete: <budget> budget <limit> hit after N runs`
  (1024 runs and 64 choice points per run by default; hitting either is exit status `2`, never a
  silent truncation). A run that fails under some order is an outcome of its own (`error: …`), a
  behavior with no choice point explores in exactly one run, and `-trace` prints the witness run's
  trace under each outcome. `-json` carries the rows as `outcomes` and the status as `exploration`.
  The REPL refuses `%schedule explore` with a typed error naming the CLI and the wire, since its
  `%action` and `%state` debuggers step one run. On the wire, `ExecuteActionResponse`,
  `ExecuteStateResponse` and `RunAnalysisResponse` gain repeated `outcomes` (observables,
  `linearizations`, `witness`, `diagnostics`, `error`) and an `exploration` status (`complete`,
  `runs`, `budgets_hit`, `runs_budget`, `depth_budget`), advertised as the `schedule_explore`
  capability beside `schedule`; the Go client adds `ExploreAction`, `ExploreState` and
  `ExploreAnalysis`, the Python client `explore_action`, `explore_state` and `explore_analysis`,
  and the Node, Java and Rust clients the capability name. A malformed spelling — `explore:`,
  `explore:runs=0`, `explore:depth=-1`, an unknown or repeated option — is refused before anything
  runs on every surface.
- **A conformance case that lists `outcomes` is now explored, and the list is exact.** The
  harness runs every such case under `explore`, failing when a listed outcome is unreachable or
  an unlisted one is reached (naming the outcome and a witness choice sequence), and when the
  budget is hit, telling the author to raise it with `"exploreBudget": {"runs": N, "depth": D}`.
  Three cases derive their outcome sets in the behavior semantic oracle: three concurrent writers
  of one feature (six linearizations, three outcomes), a decision with two overlapping guards
  inside a loop, and a state machine with two transitions enabled by one event. Exploring the
  whole suite found one case pinning a scheduling artefact — two accepts on one port, addressed
  by two sends, binding one `value` whose last writer is open — and it is restated as the two
  outcomes the oracle derives. Cases without `outcomes` are not explored, and nothing changes
  under the default schedule.
