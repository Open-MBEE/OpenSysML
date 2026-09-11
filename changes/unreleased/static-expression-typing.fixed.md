- **A bare feature reference or feature chain is typed statically by the feature it names.**
  The checker reads a name's effective scalar type — declared, given by its value, or reached
  through the features it redefines or subsets and the types it inherits, an alias followed to
  its target — so `while total { … }` over `total : Integer`, `-s` over `s : String` and
  `s == 1` are judged before execution as a literal of that type would be; the executor's own
  check now stands only for a condition whose type is genuinely unknown. The pilot corpora
  move by four diagnostics, all on `kerml-examples/Simple Tests/Expressions.kerml` and each
  adjudicated in `docs/project/pilot-differential.md`.
- **A computed value is judged against a scalar-typed feature.** An invocation's result and an
  operator expression's are checked against the feature they are bound to by the same
  classification the runtime's write conformance applies, so `attribute s : String = GetReal()`
  and `attribute s : String = a + 1.0` are reported statically and the two verdicts cannot
  disagree; numeric results still conform along the lattice, and a behavior declaring no result
  leaves the binding to the runtime. A call's result is typed by its declaration whether or not
  its input signature can be determined, so a result-only or parameterless behavior types its
  value too.
- **An argument of statically unknown type keeps every overload applicable.** A call such an
  argument leaves open selects only where one candidate remains or the known arguments prove a
  unique winner; otherwise the `invocation-ambiguous` warning `call of abs is undetermined
  between …` names the candidates left open, and the runtime settles the call — a calc's or a
  nested action's — by the values it is given: a scalar as the literal spelling it would be, so
  a positive value selects a `Natural` overload as `pick(1)` does, an object by every type it is
  classified by, an argument a candidate takes as an `expr` left unevaluated; it reports
  `ambiguous invocation` when they tie still. A settled action node holds the pins of the action
  performed alone and is read as a value by its result, not by a pin another candidate declares. The first visible candidate is no longer chosen silently. A called name is resolved as KerML 1.1
  §8.2.3.5 resolves any name — an owned declaration hides an imported one, a nested namespace's
  import stands ahead of an enclosing declaration — with no rule of its own for library
  functions.
