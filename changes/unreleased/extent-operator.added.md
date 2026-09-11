- **The extent operator `all T` evaluates.** `all T` (KerML `ExtentExpression`,
  `BaseFunctions::'all'`) answers the instances of the named type as an ordered sequence in
  declaration order, and the typer gives it the static type `T[0..*]`, judging it element by
  element as it does any collection: a condition `all Color` is refused as no Boolean, and
  `all Car as String` warns that the cast selects nothing. Because objects materialize
  lazily, the extent is the run's: for a variation definition or usage it is the variants it
  declares (`all engineChoice` in the trade-off pilot model now yields the engine alternatives,
  and its trade study proceeds to evaluation instead of stopping at the operator); for an ordinary
  definition it is every object the run has materialized or the current context reaches that the
  definition classifies, nested usages included; for an enumeration it is the declared literals.
  Any other data type, scalar (`all Integer`, `all String`) or structured (`all Point`), is
  refused with the typed `ErrUnboundedExtent`, since a run creates no data values to enumerate; an
  operand that is not a type (a package, a relationship, a comment, or no name at all) with
  `ErrTypeMismatch`, and a name that resolves to nothing with `ErrUnresolvedType`. Only nested
  usages whose type may hold an object of the type are materialized for its extent; one of those
  the extent cannot materialize ends it with that usage's error rather than an extent short of it.
  An extent that a package-level usage of several occurrences (`part wheels : Wheel[2];`) or a
  package-level port may contribute to is refused with the typed `ErrExtentUnavailable` naming the
  usage, since the runtime denotes no object of such a usage yet, rather than answered without
  them. `all T` is never model-level evaluable, so a `filter` or metadata value built on it is
  diagnosed. The native compiler keeps refusing `all` with a typed `UnsupportedError` (`operator
  'all'`), since a compiled program has no run whose extent it could report.
