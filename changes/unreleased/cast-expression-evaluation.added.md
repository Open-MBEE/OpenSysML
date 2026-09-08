- **`x as T` is evaluated.** A cast selects the values of `x` that `T` classifies, in order, and
  answers the empty sequence when none does, so `4.0 as Integer` is `4.0`, `2.5 as Integer` is
  `()` and `(1, 2.5, 3) as Integer` is `(1, 3)`. Scalars are judged by their magnitude against
  the `ScalarValues` hierarchy, quantities by whether their unit is commensurable with the
  dimension the target fixes, and objects and enumeration literals by the types they carry.
  A cast converts nothing: `ToInteger` and its siblings remain the library functions that do.
  A target that neither a value's types nor its content settles is reported rather than silently
  dropping the value.
