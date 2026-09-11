- **The extent operator `all T` evaluates.** `all T` (KerML `ExtentExpression`,
  `BaseFunctions::'all'`) answers the instances of the named type as an ordered sequence in
  declaration order, and the typer gives it the static type `T[0..*]`. Because objects
  materialize lazily, the extent is the run's: for a variation definition or usage it is the
  variants it declares (`all engineChoice` in the trade-off pilot model now yields the engine
  alternatives, and its trade study proceeds to evaluation instead of stopping at the operator);
  for an ordinary definition it is every object the run has materialized or the current context
  reaches that the definition classifies, nested usages included; for an enumeration it is the
  declared literals. A data type with no enumerated values (`all Integer`, `all String`) is
  refused with the typed `ErrUnboundedExtent`, an operand that is not a type (a package, a
  relationship, a comment, or no name at all) with `ErrTypeMismatch`, and a name that resolves to
  nothing with `ErrUnresolvedType`. An extent that a package-level usage of several occurrences
  (`part wheels : Wheel[2];`) may contribute to is refused with the typed `ErrExtentUnavailable`
  naming the usage, since the runtime denotes no object of such a usage yet, rather than answered
  without them. `all T` is never model-level evaluable, so a `filter` or metadata value built on
  it is diagnosed. The native
  compiler keeps refusing `all` with a typed `UnsupportedError` (`operator 'all'`), since a
  compiled program has no run whose extent it could report.
