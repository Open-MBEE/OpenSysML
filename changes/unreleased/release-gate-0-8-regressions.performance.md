- **The gRPC service answers `Evaluate`, `Instantiate`, `VerifyConstraint` and `Sweep` on a
  model it holds without rebuilding its resolver and semantic model per request.** A held model
  keeps a pool of idle analysis workers; a request takes one, builds one only when the pool is
  empty, and returns it with the diagnostics it added trimmed away, on the success, error and
  cancellation paths alike. Two concurrent requests still work on workers of their own. An
  `Evaluate` that had grown from 10 µs to 3.5 ms is 8 µs, `VerifyConstraint` on a held model is
  ten times faster than in 0.7.0, and the Python client's `Instantiate` on a held model is a
  third faster than 0.7.0 instead of three to eighteen times slower.
- **Writing a scalar to a typed feature no longer walks the type's difference closure or formats
  its own refusal message on every write.** Whether a type's closure subtracts anything is
  memoized per type, a write's description is built only when the write is refused, a timed
  wait formats its holder and subject only when a caller asks, and the library symbol a
  qualified name denotes is looked up once. The messages a refused write or a wait report are
  unchanged. A feature write is 11 times faster than it had become and a fifth faster than in
  0.7.0; the REPL's interpreted `SumTo(1000000)` and `Collatz(27)` calcs, action loops and
  assignment loops are back at 0.7.0's speed.
- **Lowering an action no longer computes every node's static read/write footprint up front.**
  The footprints exist for the model checker's independence relation, which is their only
  reader; they are projected once on first use and share one scan of the graph's declared
  features. Lowering a 1 000-step action chain takes 8 ms instead of 23 ms, as in 0.7.0.
- **Loading and validating a model asks the semantic model fewer repeated questions.** The
  library base a declaration's kind implies, the index order of a document's annotations and
  the engines an analysis kind dispatches to are computed once; probing whether an operand
  names a unit builds no diagnosis for the operands that do not; a scope indexes its members by
  declaration as well as by name; and the did-you-mean table takes the index's registered names
  directly. Diagnostics are unchanged. Loading a 4 000-element model in the REPL is within a
  tenth of 0.7.0's time instead of a quarter slower, and a whole-file `Analyze` is within noise
  of 0.7.0 instead of half again slower.
