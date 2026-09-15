- **A transition into a history pseudostate restores the configuration it is leaving.** The
  record a history restores is written when its owning composite state is exited, but a
  transition whose source is that owner — its self-transition or its completion transition into
  its own history — used to read the record before its exits ran, so it restored the previous
  visit's configuration (or, on the first visit, performed a default entry) instead of the one
  being left. The record is now read after the transition's exits and effects; a history's
  default transition is taken from inside the owner once it is entered, so the owner's `entry`
  runs before the default transition's effect and the region's initial transition does not run
  beside it. A `history` declared in a state machine's own body restores the machine's top-level
  configuration instead of being refused as a history outside any composite state.
- **The PSSM referee's translation no longer folds an initial transition's effect into the entry
  action of the state or region it starts.** The effect ran before the state's own `entry` and
  again on every re-entry, a history restore included; the initial transition now enters an
  empty helper state whose completion transition carries the effect. With the history fix, the
  baseline moves from 36 to 41 `pass` and 23 to 18 `fail` (History 001-A, 001-B, 001-D, 002-A
  and 002-D), adjudicated in `docs/project/pssm-referee.md`.
