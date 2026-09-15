- **A join's segments fire with their own trigger's arguments, and a refused replay undoes them
  all.** A transition into a join that accepts a payload or a call had its effect run without the
  arguments its trigger names bound, so it read the previous values or failed; each segment now
  binds what its own trigger takes from the occurrence being dispatched before its effect runs. A
  join two of whose incoming transitions leave the same region is refused when lowered — UML has
  the segments originate in different orthogonal regions, so none is an alternative to another
  — and a replay refused at a later draw among the segments undoes the segments already fired
  with the rest of the move rather than leaving some sources exited. A segment fired by a timer
  or a change condition's rise now holds the join, as one fired by a signal does, until the same
  occurrence enables every other segment into it; and a join whose sources all lie nested below
  the states of the owner's regions exits those wrappers and the owner, where before it found
  no owner and left them active. A segment leaving a composite state whose substate is active
  now holds and fires the join as the occurrence reaches that state from within, exiting the
  substate first, where before the join never fired. A join of the machine's own regions whose
  segment leaves a state nested in an orthogonal state of a region now records that region, so
  the segment exits the nested state and its wrappers once, where before the innermost region
  was recorded and the orthogonal state was exited a second time when the regions were left. A
  segment drawn among several transitions out of its source, whose join an earlier region's
  effect disarms before its turn, fires nothing and records no choice, where before the draw
  stood among the run's choices as though the segment had fired. A timer's expiry selects the
  segment it fires as a signal dispatch does — its guard holding and the join it leads into
  ready — before the route out of the join is resolved, so an expiry that does not fire the join
  reads no guard beyond it, where before a junction beyond the join with no guard holding aborted
  the run. Two time-triggered segments into a join whose timers are due at one instant fire the
  join, whichever expiry is dispatched first, where before each expiry found the other segment's
  timer to be a different occurrence and the join never fired; timers due at different instants
  still never fire it. A signal or call dispatched at the instant a segment's timer is due does
  not stand in for that expiry, so it enables no time-triggered segment. A change condition's
  rise selects the segment it fires as a signal dispatch does, the join it leads into ready,
  before the route out of the join is resolved, so a rise that does not fire the join reads no
  guard beyond it. The draw among a source's transitions that selects a segment into a join is
  recorded within the join's move, so a replay refused at a later draw among the segments undoes
  that record too, where before it stood among the run's choices after the move was undone. The
  check oracle's snapshot of a run's draws is copied rather than aliased, so a run restored to an
  earlier point no longer trims a snapshot taken after it. A rise that enables only a segment
  whose join is not ready is dispatched as a signal nothing takes is — consumed, and counted
  as one occurrence — so a run stepped under the check policy takes the dispatch it was offered,
  where before the checker offered a dispatch the poll then refused as nothing to do; and a
  segment drawn among several that fires nothing is reported as firing nothing rather than as
  a transition taken. A segment into a join that has no trigger
  is enabled by another segment's occurrence only once its source has completed — no do behavior
  of it running and, where one runs, its body done — so the join no longer fires and abandons
  that behavior; it waits for the next occurrence after the source completes. The footprint of a
  transition into a join now covers what firing the join reads and writes: every other segment's
  source, trigger and guard, the exits of every source up to the owner and of every region the
  owner (or the machine, joining its own regions) has, and the effects of every segment — so the
  checker's reduction no longer treats a step writing what a sibling segment's guard or exit
  touches as independent of the join.
