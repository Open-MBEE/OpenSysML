- **A fork may enter orthogonal regions that have no initial state.** The lowerer used to refuse
  every `parallel` region without an `entry; then <state>;`, even when a `fork`'s outgoing
  transitions were the only way in, which UML allows. Each fork's branches are now read into a
  plan — one target state per orthogonal region of one composite state, at least two branches,
  none guarded — and a region a fork enters needs no entry transition of its own; a region with
  neither is still refused with the same diagnostic. Entering through a fork runs each branch's
  effect, then the states still on the way down to the composite, then the branch's target, so a
  branch's effect precedes the composite's `entry` when the fork sits outside it, and a region no
  branch names starts at its own initial state. The PSSM referee's classifier stops filing
  *Fork 002* and *Join 001* as not expressible; both translate and run, and the baseline moves
  from 39 to 37 `not-expressible` and 18 to 20 `fail`, adjudicated in
  `docs/project/pssm-referee.md`.
