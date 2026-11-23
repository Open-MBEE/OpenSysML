- Evaluating an expression in a REPL session holding large loaded files no longer spends most of its
  time skipping the masked text: the lexer scans a whitespace run without storing its position per byte.
- Exploring schedules (`explore:` policies) allocates less when classifying due order: a move with at
  most eight places is matched by a scan instead of per-move maps.
- Reading a feature of an object under a namespace usage no longer resolves every subsetting of
  the model's namespace usages first: those are resolved when a subsetted namespace usage is read.
- A compiled calc checks its step budget once for the steps it spends before anything can fail, also
  across the temporaries of number arithmetic.
