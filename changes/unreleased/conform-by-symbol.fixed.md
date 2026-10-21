- A conformance check against a named library type — the ones every redefinition's derived
  `isVariable`/`isConstant` ask through `Usage::mayTimeVary` (`Occurrences::Occurrence`,
  `Links::SelfLink`, `Occurrences::HappensLink`, `Actions::Action`) — compares the candidate and
  its memoized supertypes to the one element registered under the name instead of building the
  qualified name of each supertype on every check, which the 0.11.0 performance record found
  costing the connector-dense `ImportValues` benchmarks 29% more allocations (135.5 k → 175.2 k
  per load) and a synthetic document's analysis 10% more. A name no or several elements carry
  is still compared by name.
