- **`-engine check` searches every schedule of a state machine, and of actions and machines
  on one clock.** The `check` engine takes an *invocation* — every `-action` and `-state`
  named (`%action`, `%state`, `RunFor` in the REPL), started on one context and run to the
  `-advance` horizon, together with the machines of the objects they materialize — and searches
  it as one run, one move at a time: a token advancing one node, an event dispatched, a `do`
  behavior stepped. Beside the action choices it explored before, the search draws every
  transition enabled for one event, every order of the regions reacting to it, every branch of
  a choice pseudostate, every order of the events the library leaves unordered at one instant
  (two timers firing together, a timer beside a signal of the same timestamp; completion
  events still go first and signals arrive in the order sent) and every order of the executors
  due at one instant — an action's `accept after` and a machine's timer falling due together
  being the case a run under one policy silently decides. A machine resting where nothing will
  wake it is a complete schedule, not a deadlock; an action left incomplete is one as before; a
  wait past the horizon is left unreached and the verdict reads `exhaustive up to t=<horizon>`.
  `finalState` is an observable beside the attributes — `-check-diverge finalState`, and
  `<behavior>.<feature>` or `<behavior> finalState` when several behaviors are checked
  together — so a machine that rests in different states on different schedules is *divergent*
  with a witness per state, and every witness of a joint run replays through `-schedule
  replay:` and `%replay` onto `%action`, `%state` and `%advance` on one object. Moves of
  independent footprints — a dispatch's being the guards, triggers, effects and state
  activities of the transitions the event can select — are searched in one order only, as an
  action's already were. Without `-advance` each behavior named is its own search; under
  `-engine smt` a `-state` or `-advance` is refused, as the solver does not encode a machine.
- **A body paused mid-statement is state a snapshot captures.** A token suspended at a
  breakpoint or on the clock inside a block or loop, a performed action waiting on its callee,
  and a state's `do` behavior waiting at an `accept` or a timed wait are held as explicit
  continuations — the statement cursor, the block, loop and flow-node frames, the nested
  performance and the wait — in place of a suspended coroutine, so `Snapshot` and `Restore`
  round-trip them and the checker searches the cases that pause a body like any other. A
  portable image (`HeldImage`, what a sweep of a held object copies) still refuses one with
  `ErrSnapshotPausedBody`, since its continuation points into the model's lowered statements.
  Every existing run, trace and choice point is unchanged.
