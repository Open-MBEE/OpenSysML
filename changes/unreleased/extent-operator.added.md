- **The extent operator `all T` evaluates.** `all T` (KerML `ExtentExpression`,
  `BaseFunctions::'all'`) answers the instances of the named type as an ordered sequence in
  declaration order, and the typer gives it the static type `T[0..*]`, judging it element by
  element as it does any collection where a collection binds (`attribute xs : Boolean[*] = all
  Flags;`), while a condition `all T` is refused for any `T`, Boolean-typed included, as the
  sequence it is rather than the one Boolean a condition needs — as is an extent given to a Boolean
  operator (`not all Flags`, `all Flags and true`) — and `all Car as String` warns
  that the cast selects nothing. Because objects materialize
  lazily, the extent is the run's: for a variation definition or usage it is the variants it
  declares (`all engineChoice` in the trade-off pilot model now yields the engine alternatives,
  and its trade study proceeds to evaluation instead of stopping at the operator); for an ordinary
  definition it is every object the run has materialized or the current context reaches that the
  definition classifies, nested usages included; for an enumeration it is the declared literals.
  Any other data type, scalar (`all Integer`, `all String`) or structured (`all Point`), is
  refused with the typed `ErrUnboundedExtent`, since a run creates no data values to enumerate; an
  operand that is not a type (a package, a relationship, a comment, or no name at all) with
  `ErrTypeMismatch`, and a name that resolves to nothing with `ErrUnresolvedType`. Only nested
  usages whose type may hold an object of the type are materialized for its extent, and of those
  every one but a usage that would create another object of a declaration already on the path,
  settled by its value's possible types where they agree and else by what reading it makes, a
  read making one undone (so a composition recursing through one declaration ends, while each
  object a run linked to another of its declaration still has its own nested usages read and a
  value choosing at run time between recursing and not contributes what it chose); one the extent
  cannot materialize ends it with that usage's error rather than an extent short of it, and one
  making an object on the path together with one that may lead to a `T` is undone and refused
  with `ErrExtentUnavailable` naming it.
  An extent that a package-level port may contribute to is refused with the typed
  `ErrExtentUnavailable` naming the usage, since the runtime denotes no object of such a usage,
  rather than answered without it. A namespace-level object usage given a value (`ref part car : Car = new Car();`,
  `part fleet = new Truck();`, `ref part alias : Car = spare;`) is bound to that value for the run —
  a feature value binds its feature to its expression's result (KerML 1.0 §7.4.11 Feature Values,
  §8.4.4.11) — so `all Car` reaches the object it denotes before any read of it, and the usage
  denotes that one object on every read; it used to be evaluated anew on each read, so `car` read
  twice was two `Car`s. A `default` or initial (`:=`) value at namespace level is held for the run
  the same way, there being no other individual for it to be realized on; a `default` nested in a
  definition is still what each object built from it reads at construction. A usage bound to an
  extent of its own type (`ref part cars : Car[*] = all Car;`) stands for no object while it is
  being bound, and one whose value depends on it for none yet, so the extent binds to the objects
  there are; a value reaching back to its own usage is refused as a cyclic feature value. A binding
  refused after constructing objects (`ref part car : Car = new Boat();`) leaves none of them
  behind, however often the usage is read. `all T`
  is never model-level evaluable, so a `filter` or metadata value built on it is diagnosed. The
  native compiler keeps refusing `all` with a typed `UnsupportedError` (`operator
  'all'`), since a compiled program has no run whose extent it could report.
