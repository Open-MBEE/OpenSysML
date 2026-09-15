- **A junction with several enabled outgoing branches draws one of them as a choice point.**
  The guards of a junction's outgoing transitions are still read before the incoming transition
  fires, against the data as it then stands, but where several hold the runtime used to take the
  first in declaration order; it now draws and records the transition choice point at the junction,
  as it does at a choice, so a `seed` policy replays its draw, `explore` enumerates every branch, and
  the trace and the `choice` note name the junction. The draw is made only as the transition fires
  — after the order among several regions' transitions is drawn and the transition's own guard is
  read again — so a witness lists the region order before the junction's draw and a transition
  another region's effect disarms draws nothing; a history's default transition through such a
  junction records its draw the same way. The unguarded branches remain the default
  when no guard holds, and a junction with no enabled branch still leaves the compound transition
  unenabled. The PSSM referee's baseline moves from 44 to 45 `pass` and 16 to 15 `fail`
  (Junction 003), adjudicated in `docs/project/pssm-referee.md`.
