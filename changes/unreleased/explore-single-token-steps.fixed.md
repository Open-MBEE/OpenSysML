- **`explore` advances one token per step, so `complete` covers every interleaving.** Exploration
  used to permute the tokens of one lockstep step, in which every steppable token moved once, so a
  branch of two nodes could never both run before a concurrent branch's one node: a fork of
  `left1 { x := 1 } → left2 { y := x }` against `right { x := 2 }` reported `complete` with two
  outcomes and missed `x = 2, y = 1`. Under `explore` a step is now one token advancing one node,
  the tokens able to act are picked among afresh after each move, and each pick is its own
  `step N:` choice point in the witness; the fixed policies (`reverse`, `declared`, `seed:<n>`)
  keep their sweep, so no default trace changed. Run counts grow with the finer granularity
  (`action_merge_fork_branch_and_loop` needs `explore:runs=10000` to complete) and the semantic
  oracle's figures are re-derived; `action_explore_write_between_branch_nodes` pins the case.
  A performed action paused on the clock is among the tokens an exploring step picks from once
  its wait has ended, so a sibling accept due at the same instant no longer always runs first:
  `action_explore_performed_and_accept_due_together` reaches both writes, six linearizations.
