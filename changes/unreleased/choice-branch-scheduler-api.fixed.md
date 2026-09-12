- **A state machine with a choice pseudostate runs again.** A choice whose guards are read
  after the incoming transition's effect drew its branch through the scheduler's former
  pick-then-describe calls, which the replay scheduler replaced with one `choose` over the
  choice point; the runtime did not build. The branch is now drawn as a state's conflicting
  transitions are: the choice point is built with its enabled branches in declaration order,
  the scheduling policy (declared, a seed, `explore`, a replay) takes one, and the run notes
  the choice with the branch taken and its declaration. The recorded choice line is unchanged,
  an exploration's witness over such a choice replays to its outcome, and every choice
  fixture and trace keeps its expectation. A replay whose witness names a branch the choice
  does not enable is refused with the machine as the occurrence found it: the compound
  transition's exits and incoming effects are undone, no branch is taken and none is recorded,
  as a refused transition or decision move changes nothing.
