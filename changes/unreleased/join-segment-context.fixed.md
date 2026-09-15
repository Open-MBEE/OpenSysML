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
  still never fire it.
