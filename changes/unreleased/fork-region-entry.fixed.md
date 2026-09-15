- **A fork may enter orthogonal regions that have no initial state.** The lowerer used to refuse
  every `parallel` region without an `entry; then <state>;`, even when a `fork`'s outgoing
  transitions were the only way in, which UML allows. Each fork's branches are now read into a
  plan — one target state per orthogonal region of one composite state, at least two branches,
  none guarded — and a region a fork enters needs no entry transition of its own; a region with
  neither is still refused with the same diagnostic, and so is a machine with another way into
  the composite state — a transition to the state itself, to another of its regions or to its
  history, or the machine's entry naming it — since that way would start the region by default
  and it has no default start. Entering through a fork leaves the source configuration down to
  the ancestor the source and the composite share, as a move to a single state does, so an
  active ancestor is neither exited nor entered again; then each branch runs its effect, enters
  the states still on the way down to the composite, then its target, so a branch's effect
  precedes the composite's `entry` when the fork sits outside it, and a region no branch names
  starts at its own initial state. A composite state entered on the way down does not start the
  region the branches pass through at its own initial state — only its other regions start as
  usual — and a do behavior in a region the fork leaves untouched still takes the occurrence
  that fired it. The PSSM referee's classifier stops filing
  *Fork 002* and *Join 001* as not expressible; both translate and run, and the baseline moves
  from 39 to 37 `not-expressible` and 18 to 20 `fail`, adjudicated in
  `docs/project/pssm-referee.md`.
