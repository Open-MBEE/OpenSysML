- **A state's `do` behavior may be a full action: one that waits on the clock or for a
  signal, and a typed usage with pin bindings.** `do action poll { action wait accept after
  3 [SI::s]; then action count assign ticks := ticks + 1; }` was refused at instantiation with
  `statement not executable: … action usage "wait" in a body is not executable`, and `do action
  poll : Poll { inout n = ticks; }` with `performing an action and stating a body of its own in a
  body is not executable`, although both validate clean. A do body's flow now starts at its one
  node no succession leads to where no `first` says (two such nodes or a cycle are still reported,
  naming them), and the body runs as one performance that may pause: an `accept after`/`accept at`
  parks it on the shared clock — `%advance`/`-advance` move it and list it under `Waiting on the
  clock` — and an `accept Sig` parks it until a matching signal is sent (`%send Sig` takes it,
  reporting `the do behavior of state <s> goes on from its accept`, rather than refusing it because
  no transition fires; a signal sent from a sibling object wakes it too), while the machine's
  transitions and its other regions' do behaviors go on around it. `%send`'s preview and the
  dispatch select the signal's takers by one rule: the transition chosen for a state is drawn once,
  before the takers are settled, and where it leaves the state whose do behavior is parked for the
  signal it is the only taker there, while a transition between that state's own substates, or
  one in a sibling region, shares the one dispatch with the do behavior; the step reports `Event dispatched,
  letting the do behavior of state <s> go on from its accept`, and such a signal is neither deferred
  nor counted as dropped. A nested action node stating its flow in declaration order starts at its one
  unpreceded node as the body does, rather than being reported as a flow without a start. Leaving the state ends the
  performance: its wait leaves the clock, nothing after the wait runs, and a signal sent later
  wakes nothing. A typed usage whose body declares only the pins of the action it performs
  performs that action, an `inout` pin bound to a feature (`inout n = ticks`) writing back when the
  performance ends and not when the state's exit abandons it, and one valued by an enumeration
  literal or another name no enclosing feature answers (`inout mode = Mode::idle`) starting from
  that value and writing nowhere, rather than being refused as an output bound to no feature; an
  `in` pin nothing binds, or one
  bound to a feature the state does not declare, is a typed error naming the pin. Which of two
  regions' do behaviors due at one instant acts first in a round is a choice point (`choice do
  round at t=2.0: states lwork, rwork react`), explored and seeded as the other choice points are.
  An `entry` or `exit` body, or a transition effect, whose flow waits on the clock is refused with
  `state behavior waits for the clock` — those behaviors are performed whole at the instant they
  are triggered.
