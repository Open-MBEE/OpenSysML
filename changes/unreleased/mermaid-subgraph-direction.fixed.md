- **Every Mermaid flowchart `subgraph` now states the flowchart's `direction`.** Mermaid lays
  out a subgraph that states no direction without regard to the flowchart's, so an action
  rendering declared `flowchart TD` drew its container's contents left to right, and an
  interconnection or action rendering with `direction BT` or `RL` lost the direction inside
  every container. Each `subgraph`, nested ones included, now opens on `direction <flow>` —
  `TD`, `LR` for an interconnection, or the direction the view or the caller asked for — so
  the drawing follows the declared direction throughout. A tree draws containment as edges
  rather than subgraphs and is unchanged.
