- **A body inside a nested definition no longer reaches the enclosing definition's features
  by their bare names.** `part def P { attribute n = 1; calc def E { n + 1 } }` — and the same
  shape with a `constraint def`, an `action def`'s `assign`/`if`, or a `state def`'s transition
  guard — now reports `Must be an accessible feature (use dot notation for nesting)`, as the
  reference implementation does: a nested *definition* is a new type with no featuring
  relationship to the one that owns it, so `n` is a feature of `P`, not of `E`. The featuring
  contexts of a definition were being derived from its owner as if it were a feature. A nested
  *usage* (`calc e { n + 1 }`) is featured by `P` and still reaches `n`, and a nested definition
  still reaches its own, inherited and redefined features and every package-level feature.
