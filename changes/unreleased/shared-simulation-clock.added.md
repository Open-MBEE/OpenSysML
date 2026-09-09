- **One simulation clock, owned by the runtime context, that actions and state machines wait
  on together.** Simulation time is a property of the runtime a behavior runs in, no longer of
  one state machine's executor: every state machine and action a context runs — the ones
  `%action`/`%state` debug, the ones an object exhibits, the ones a body performs — reads the same
  clock (`Context.Clock()`, in `SI::s`), so two machines materialized in one context share time,
  and a nested performance runs on the enclosing clock rather than a copy. An action body now
  waits on it: `accept after <duration>` parks the token until the clock has moved that far from
  the moment the accept was reached, `accept at <instant>` until the clock reads the instant (one
  already passed is due at once), with the unit conversion and the `DurationValue`/`TimeInstantValue`
  refusal a transition's trigger gets; an action running on its own moves the clock to each wait
  as it reaches it. Advancing moves everything: `Context.Advance(duration)` runs every state
  event, action token, change-condition poll and do round due up to the new instant, instant by
  instant, within the event, do-step and step budgets, and returns what it moved. `-advance` no
  longer needs `-state`: it runs the invocation's `-action` and `-state` behaviors together on the
  one clock (an action sending a signal a machine accepts after a delay), and an action still
  parked on the clock when the time is up is reported as undecided with the instant it waits for;
  `%advance` moves the clock of the session's runtime, so an `%action` debugger parked at
  `accept after 5 [SI::s]` and a `%state` debugger both move and the report covers each, and
  `%step` on a token waiting only on time says so and names the `%advance` that would move it.
  Which executor runs first when several are due at one instant — an action token and a state
  transition, two machines, two actions — is a new choice point, `due order`, drawn by the
  scheduling policy (`choice at t=5.0: due action watcher, state machine blinking of object #1
  (unordered; ran state machine blinking of object #1 first)`): the executor started last runs
  first under the default `reverse`, the first started under `declared`, a draw under `seed:<n>`;
  one executor alone due is no choice and is not reported, so every existing single-behavior
  result and trace is unchanged. `ExecuteActionResponse` and `ExecuteStateResponse` report
  `final_time`, the clock's reading when the run ended, advertised as the `final_time` capability
  (`CAPABILITY_FINAL_TIME`, `Capabilities.FINAL_TIME`) and read by the Python client's
  `execute_state` result and by the generated response types of the Node, Java and Rust clients. The
  robustness case that pinned the old refusal of a time trigger in an action body
  (`action_accept_time_trigger`) is replaced by `action_accept_time_waits` and `clock_advance`,
  which pin the waits firing, a negative, infinite or not-a-number duration and one leading past
  the last instant the clock can hold (each refused, the clock unmoved), a duration of another
  dimension, an instant already past, a wait beyond the advance, an advance of zero, one with
  nothing waiting, and a wait met in a flow a body runs or in an action a node performs, which
  pauses the token's work until the clock reaches it rather than moving the clock past a bounded
  advance's deadline.
