- **A deferred event outranks a transition in an enclosing state or a sibling region.** While a
  state that defers an event is active, the event is held back from every enabled transition
  except one whose source is that state or a state nested in it; a transition in an enclosing
  state or in a sibling orthogonal region waits until the deferring state is exited, and the
  event is then dispatched, ahead of later arrivals, to the configuration that exit leaves. Only
  a nested transition overrides the deferral and consumes the event, and with two regions each
  deferring it, the event fires only if every deferring state has such a nested transition.
  Before, a sibling region's transition fired on the event and the deferral was never
  consulted. The outcome is determined by the configuration and is not a choice point.
- **A history with nothing to restore performs the owning state's ordinary entry.** A `history`
  or `deep history` that has no recorded configuration and no outgoing default transition now
  enters the owning state as a first entry would, through its `entry; then …;`, instead of
  failing the run; a region left through `done` records no history and is re-entered the same
  way. An owner with neither a default transition nor an entry transition fails with the typed
  `ErrHistoryWithoutEntry` naming the history.
- **A composite state's completion fires the composite's own completion transitions; the machine
  ends only when its top-level regions complete.** `then done;` inside a composite state's body
  now ends that state: once its do behavior and every region have ended, its transitions with no
  trigger are queued as completion events at the current instant, ordered as a plain state's are,
  so `state outer { … then done; } transition first outer then next;` enters `next`. Before, a
  nested `done` completed the whole machine and the composite's completion transition never
  fired. A completed composite with no enabled completion transition stays active and the
  machine runs on until its own top-level regions reach `done`.
- **A `choice` reads its guards after the incoming transition's effect; a `junction` before.**
  A route through a choice is now resolved on arrival: the states every branch leaves are
  exited, the effects into the choice run, and only then are its guards read, so `transition
  first idle do assign x := 1 then pick; transition first pick if x == 1 then seen;` reaches
  `seen`. Several enabled branches are the existing transition choice point, which `explore`
  enumerates and `seed:<n>` replays; no enabled branch fails the run with the typed
  `ErrChoiceWithoutBranch` naming the choice. A junction's guards are still read before the
  transition fires, so a junction with no holding guard leaves the transition not enabled, and on
  a chain each pseudostate follows its own rule at the point the route reaches it.
