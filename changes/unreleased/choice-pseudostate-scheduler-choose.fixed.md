- **The runtime builds again, and a choice pseudostate follows the scheduling policy.** A
  choice with several enabled branches resolved its pick through two scheduler methods the
  replay policy had replaced, so `internal/core/runtime` no longer compiled. The choice now
  goes through the scheduler's one `choose` entry like every other choice point, so a seeded,
  explored or replayed run resolves a choice pseudostate the same way it resolves a state's
  competing transitions — and a witness naming a branch the choice's guards do not enable is
  refused as `replay refused: ... is not enabled` instead of silently taking the first branch.
