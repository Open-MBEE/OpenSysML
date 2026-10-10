- The steps of an analysis or verification case that states no succession are now unordered, as an
  action definition's subactions are: `explore`, `-engine check`, replay and seeded runs reach every
  interleaving of them instead of only declaration order, so a case such as
  `action s1 { assign x := x * 10; } action s2 { assign x := x + 2; }` reports both `12` and `30`.
  `declared` and the default `reverse` schedule still perform the steps in declaration order, so
  default results do not change. A case inherited as an action step keeps its case-body lowering and
  declaring scope.
