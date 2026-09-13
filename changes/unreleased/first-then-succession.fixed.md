- **A two-ended `first a then b;` in an action body is the succession a → b, not a second start.**
  Lowering used to collect every `first` member as the action's initial node, so a body writing
  `first start;` beside one or more `first a then b;` successions — as OMG's
  `3a-Function-based Behavior-2` and the Annex A vehicle models do — validated clean and then
  failed at `%action` with `action has multiple initial nodes`. A two-ended `first`, with or
  without a body, now lowers to the same edge `succession first a then b;` does, any number of
  times and beside `first start;`, `then` chains, forks, joins, decisions and merges; only the
  one-ended `first start;` and `first a;` mark where the flow starts, two of them are still
  rejected, and a body writing neither still starts at its one node no succession leads to. The
  validation pass checks both ends of a two-ended `first`, and the RDF mapping is unchanged.
