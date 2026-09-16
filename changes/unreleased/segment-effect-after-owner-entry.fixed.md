- **A segment leaving a junction or choice declared inside a composite state runs its effect
  after that state's entry.** A compound transition used to run every effect of its route after
  its exits and before any state on the way down to the target was entered, so a segment out of
  a pseudostate declared in a composite state — the composite being entered on the way to the
  pseudostate — logged its effect before the composite's `entry`. Each effect now runs once the
  states down to the one declaring the pseudostate it leaves are entered: the composite's
  `entry`, then the segment's effect, then the entry of the target below, at every depth of
  nesting, for a junction as for a choice (whose guards are still read after the effects into
  it), for a pseudostate in one region of a parallel state (the parallel state entered first, the
  other regions starting as usual) and for a history's default transition through such a
  pseudostate. A choice whose branches end in different states enters only the states every
  branch enters before its guards are read. The PSSM referee's counts do not move: *Junction 005*
  now reaches an admitted trace and misses only the interleavings of the other region's entry,
  so it stays `fail` on the region-order gap alone, adjudicated in `docs/project/pssm-referee.md`.
