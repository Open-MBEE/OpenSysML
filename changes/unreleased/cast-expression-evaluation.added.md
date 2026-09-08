- **`x as T` is evaluated.** A cast selects the values of `x` that `T` classifies, in order, and
  answers the empty sequence when none does, so `4.0 as Integer` is `4.0`, `2.5 as Integer` is
  `()` and `(1, 2.5, 3) as Integer` is `(1, 3)`. Scalars are judged by their magnitude against
  the `ScalarValues` hierarchy, quantities by whether their unit is commensurable with the
  dimension the target fixes, arrays, vectors, vector and tensor quantities, measurement
  references, frames and transformations by their shape, units and frame, and objects and
  enumeration literals by the types they carry.
  A type composed of others classifies as they do — the values of a union are those of any of the
  types it unions, of an intersection those of every type it intersects, of a difference those of
  the first that are none of the rest, however deeply nested — so a cast to one keeps them, the
  feature it is written to holds them, and `istype` answers for them; casting a value of a union to
  one of its members is not reported as unrelated either, nor is casting a value to a type composed
  of one it relates to.
  A composed target a value's types leave open is read through its operands, so a bare quantity
  cast to a union of quantity types is kept by the operand whose reference its unit matches, and an
  operand the value settles nothing about is reported as undecided only where no other operand
  excludes the value outright.
  A composed type weighs all the types a value is of at once, whether they are the types a runtime
  value carries or those its feature is declared with, so an object held as a type a
  difference subtracts is none of its values, whether the difference is the target, one it
  specializes, or one an intersection of it reaches.
  Every type a value's feature is declared with counts among the types it is of, so a custom scalar
  subtype (`attribute e : Even = 4`) and a scalar-valued enumeration keep the values declared with
  them — a written sequence entry by entry, each judged by its own declaration however many values
  it holds and however deeply nested, so `(GradePoints::a, GradePoints::b) as GradePoints` keeps
  both and no entry is judged by another's type — and a quantity subtype narrowing its dimension by something a magnitude and a unit do not
  state keeps a value declared with it. An expression written as a value is kept by the evaluation
  type it is read as, a boolean body by `BooleanEvaluation`.
  A cast converts nothing: `ToInteger` and its siblings remain the library functions that do.
  A target that neither a value's types nor its content settles is reported rather than silently
  dropping the value.
- **Classifying a value is model-level evaluable.** `as`, `istype` and `hastype` read the type they
  name rather than folding their operand, so a metadata body may bind `x = 1 as Integer`.
