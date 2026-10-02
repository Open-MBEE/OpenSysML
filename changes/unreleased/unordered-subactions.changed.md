- **Subactions no succession orders are performed instead of refused.** An action's composite
  action usages that no succession reaches — action nodes and `send`, `accept`, `assign`, `if`,
  `while` and `for` written among its members — start with the action's performance, unordered
  against each other and beside any `first`-rooted flow, and the action ends only once all have
  (`Actions::subactions :> subperformances`, `Performances::enclosedPerformances`). An action with
  two such nodes, or a bare `send`, no longer fails with "no initial node found" or "has no position
  in the token flow"; `-schedule explore` enumerates their interleavings. `ref` and abstract usages
  are not performed, and the bounded model checker refuses such a flow as not encoded.
