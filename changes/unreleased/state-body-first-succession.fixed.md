- **A state body's two-ended `first a then b;` is the succession `succession first a then b;`
  spells with its keyword: a completion transition from `a` to `b`.** It was read as an initial
  node and lowered as an entry transition into `b`, so a machine with no `entry` marker silently
  started in `b`, and one written `entry; then a; first a then b; first b then c;` never left `a`.
  The keyword-less spelling now parses as the same `SuccessionAsUsage`, resolves nested, qualified,
  region-local and pseudostate ends as a `succession` does, and designates no initial state; a
  machine whose only edges are such successions is reported as having no initial state, as any
  exhibited machine without one is. `first start then off;`, leaving the `start` every state
  inherits, still designates where the machine starts. A one-ended `first a;` in a state body
  orders nothing and is reported (`first-names-no-target`) rather than ignored. Validation and the
  RDF mapping follow: the two-ended form is checked and exported as a succession, with no
  `sysx:InitialNode` written for it.
- **A succession end written as a feature chain (`succession first b then c.c1;`, `first b then
  c.c1;`) names the same nested vertex as `c::c1`.** The chained end was dropped, so the edge into
  `c1` was never lowered; it is now resolved through the endpoint lookup as the qualified spelling
  is, at either end of the edge, its first segment reaching a vertex nested anywhere in the machine
  as a qualified end's does, and a chain whose member or operand is not a state or pseudostate is
  reported.
