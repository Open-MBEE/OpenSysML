- **Compiled calcs pass their Integers, Rationals, Reals and Booleans in three words.** The
  compiled calc tier held each scalar in 56 bytes and, for every parameter, local and result,
  boxed it back into the evaluator's constant to decide whether the declaration holds it, and
  built the diagnostic a refusal would name whether or not one followed. A scalar is now the
  constant packed into 24 bytes (`semantics.Packed`), unpacked without allocating; which kinds
  a declaration holds whatever the value is decided once when the calc is compiled; the
  diagnostic is built only on refusal; and the step charge inlines into every node.
  `Fib(25)` interpreted runs in a fifth of the time with a quarter of the allocations; every
  result, promotion to a big value, refusal, error message and step count is unchanged.
