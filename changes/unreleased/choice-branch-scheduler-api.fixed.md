- **A replay refused at a choice pseudostate leaves the machine as the occurrence found it.**
  A choice's branch is drawn after the compound transition has left the states every branch
  leaves and run the incoming segments' effects, so a replay whose witness named a branch the
  choice does not enable was refused with those exits and effects standing. The refusal now
  undoes the move whole: the exits, their effects, the incoming effects, the trace and the
  run's notes are restored and a do behavior an undone exit abandoned stays paused where it
  was — no branch is taken and no choice recorded, as a refused transition or decision move
  changes nothing. An exploration's witness over such a choice replays to its outcome.
