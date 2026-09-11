- **A literal's direct type is its `ScalarValues` definition, read from the library and not from
  the evaluating scope.** `directValueType` answers an integer with `ScalarValues::Integer`, a
  finite real with `ScalarValues::Rational` (KerML 1.0 §8.4.4.9.2), a Boolean, string or complex
  with its library type, found by qualified name the way `*` is found as `Positive`, where it
  resolved the bare simple name in scope — undetermined in a scope importing nothing from
  `ScalarValues`, and a model's own `attribute def Integer` where one was declared. A written
  `istype Integer` still resolves to the type the scope sees, so `2 istype Integer` beside such a
  declaration is `false` and `2 istype ScalarValues::Integer` `true`, as the pilot answers. A model
  built without the library keeps its same-named types as the stand-in.
