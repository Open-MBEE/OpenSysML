- **The runtime builds again.** A choice pseudostate with several enabled branches resolved
  its pick through two scheduler methods the replay policy had replaced, so
  `internal/core/runtime` no longer compiled. The choice now goes through the scheduler's one
  `choose` entry like every other choice point, so a seeded, explored or replayed run resolves
  a choice pseudostate the same way it resolves a state's competing transitions.
