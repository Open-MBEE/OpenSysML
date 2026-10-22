- Enumerating what a wildcard import brings — the references index, rename, completion and
  every unqualified lookup that imports feed — no longer pays, per member, for the hidden-import
  machinery when nothing is hidden: a member re-exported through another namespace is checked
  against the memoized `hidesBelow` before any route is walked, its qualified name is compared
  segment by segment instead of being built, and the routes gating it are visited in place
  instead of copied into a list the lookup then reads once. The language server's cold
  references query and warm rename of the 0.11.0 performance record go from 6.28 M and 12.4 M
  allocations to 1.94 M and 3.74 M; a member re-exported through a second wildcard import costs
  2 allocations to enumerate instead of 6.
