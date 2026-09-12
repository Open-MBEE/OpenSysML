- **`-engine check` searches every schedule of an action.** The `check` engine is an
  explicit-state model checker over the action `-action`/`%action` names: it takes the run one
  move at a time — one token advancing one node, a body being one move — snapshots the executor
  before each choice and backtracks to take every other, so every schedule the library admits is
  visited and no other, and reports what it finds: a constraint or requirement named with
  `-check-property <name>` (`%check-property`) that is `false` at a stable state, a deadlock or
  a typed error a body raises, each as a *violation* on the schedule that reaches it; a feature
  that ends with different final values on different schedules as *divergent* with every value
  it takes (`-check-diverge <feature>`, `%check-diverge`; by default every attribute of the
  action and of its performing object, an action run without one on its own attributes);
  otherwise `no violation, exhaustive` when the search finished, or `no violation within bounds`
  naming every bound it reached — `-check-depth` (moves along one schedule, 10 000),
  `-check-states` (distinct states, 1 000 000), `-check-timeout` (the plan's clock; a search it
  stops is reported `incomplete: time`, not as a verdict) and the executor's own budgets, set
  in the REPL with `%check-bounds`. Two moves whose statically computed footprints are
  independent are searched in one order only, and a state already visited is not searched
  again. `-check-witness <dir>` writes one file per violation and divergent value, named for
  the action and the object performing it — the schedule's choice lines, a blank line, then
  the run's trace — and each is reported
  *witnessed* only after the interpreter replayed it to the state it claims; `-schedule
  replay:<file>` and `%replay <witness>` step that run under the ordinary debugger. The CLI
  exits `1` on a violation or a divergence, `0` on an exhaustive clean search and `2` on a
  bounded, cancelled or refused one; `-json` carries a `check` object (`verdict`, `states`,
  `moves`, `depth`, `boundsHit`, `violations[]`, `divergent[]`, `outcomes[]` and the witness
  paths) on the engine's `results[]` entry. The engine is listed by `-engines` and `%engines`
  at authority *bounded*, `auto` never picks it over `explore`, and it refuses with a typed
  reason what it does not search: a state machine, a body paused mid-statement (an `accept` or
  a timed wait inside a block), a state and an action due together, and a `-check-*` flag
  without `-engine check` or the engine without an action. State machines, `do` interleaving
  and checking across objects are later stages'; the search itself is single-threaded, `-jobs`
  dividing only the replay of its witnesses.
