- **`x as T` is evaluated.** A cast selects the values of `x` that `T` classifies, in order, and
  answers the empty sequence when none does, so `4.0 as Integer` is `4.0`, `2.5 as Integer` is
  `()` and `(1, 2.5, 3) as Integer` is `(1, 3)`. Scalars are judged by their magnitude against
  the `ScalarValues` hierarchy, quantities by whether their unit is commensurable with the
  dimension the target fixes, arrays, vectors, vector and tensor quantities, measurement
  references, frames and transformations by their shape, units and frame, and objects and
  enumeration literals by the types they carry.
  The type a value's feature is declared with counts among the types it is of, so a custom scalar
  subtype (`attribute e : Even = 4`) and a scalar-valued enumeration keep the values declared with
  them, and a quantity subtype narrowing its dimension by something a magnitude and a unit do not
  state keeps a value declared with it. An expression written as a value is kept by the evaluation
  type it is read as, a boolean body by `BooleanEvaluation`.
  A cast converts nothing: `ToInteger` and its siblings remain the library functions that do.
  A target that neither a value's types nor its content settles is reported rather than silently
  dropping the value.
- **Classifying a value is model-level evaluable.** `as`, `istype` and `hastype` read the type they
  name rather than folding their operand, so a metadata body may bind `x = 1 as Integer`.
