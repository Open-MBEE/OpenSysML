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
  no owner and left them active.
