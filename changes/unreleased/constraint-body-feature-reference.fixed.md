- **A constraint body whose condition is a bare feature reference comes back from its RDF
  graph alone.** `require constraint { ready }`, `assert constraint { ready }`, a bare
  `constraint { ready }`, a named `require constraint <'R-1'> ok { ready }` and KerML's
  `inv { ready }` are carried as a `sysml:FeatureReferenceExpression` with its `sysml:referent`,
  but the notation written from a graph with no `sysx:sourceText` closed the condition with a
  `;` — and `ready;` declares a feature named `ready` rather than referring to one, so
  `sysml -convert sysml -from ttl` refused the model (`the notation written for it does not
  read back as a reference`). The condition that closes a constraint body is now written bare,
  as a calculation's result expression is, for every condition shape (`not x`, `a and b`, a
  comparison); a condition that others follow keeps its `;`. The rebuilt notation validates
  as the original does and a second Turtle hop states the same triples, so
  `examples/phase-c-behavioral-bodies.sysml` now converts from its structure alone. The
  `.canonical.golden.sysml` fixtures under `internal/core/export/testdata/convert/` lose the `;`
  after their trailing conditions accordingly.
