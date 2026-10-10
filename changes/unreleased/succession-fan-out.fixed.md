- **Every target of several successions out of an ordinary action node is performed.** A
  succession is a `HappensBefore` link that orders its source before its target and excludes no
  other target, so `first a then b; first a then c;` now performs both `b` and `c` after `a`,
  unordered relative to each other, as a fork would; a guarded succession whose guard is false
  leaves out only its own link, and two holding guards out of a non-decision node follow both
  instead of failing with "more than one succession is enabled". The same holds for statement
  nodes (`if`, `while`, `loop`, `for`, `assign`, `send`), the start node and a `[0]` step. Only a
  `decide` node chooses one branch; a `join` or `merge` with several outgoing successions is still
  refused, as their validation constraints require, and a repeated step still needs written end
  multiplicities on each edge out of it. The `smt` engine encodes the fan-out as it encodes a fork.
