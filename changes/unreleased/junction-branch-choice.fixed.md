- **A junction with several enabled outgoing branches draws one of them as a choice point.**
  The guards of a junction's outgoing transitions are still read before the incoming transition
  fires, against the data as it then stands, but where several hold the runtime used to take the
  first in declaration order; it now records the transition choice point at the junction, as it
  does at a choice, so a `seed` policy replays its draw, `explore` enumerates every branch, and
  the trace and the `choice` note name the junction. The unguarded branches remain the default
  when no guard holds, and a junction with no enabled branch still leaves the compound transition
  unenabled. The PSSM referee's baseline moves from 41 to 42 `pass` and 18 to 17 `fail`
  (Junction 003), adjudicated in `docs/project/pssm-referee.md`.
