- **A model-level expression over a feature the model leaves open is `<undetermined>`, not an
  error and not a made-up number.** Evaluated at model level (`-e`, `%eval`, gRPC `Evaluate`),
  a reference to an attribute with no value (`attribute u;`) failed the whole expression with
  `no value for feature u`, even where the other operand fixed the answer, and a feature whose
  count the model does not fix was counted at its minimum, so `size(gear)` for a `part gear[1..*]`
  was `1` and `isEmpty(loose)` for a `part loose[0..2]` was `true`. Such a read is now the value
  `<undetermined>`, which every expression over it carries (`u + 5`, `u > 3`, `-u`,
  `if u > 0 ? 1 else 2`, `(10, 20, 30)#(u)`, `size(gear)`, `isEmpty(loose)`, `gear#(7)`), while
  what the model fixes still answers: KerML's conditional `and`, `or` and `implies`
  (`ControlFunctions`) decide on a constant second operand as they already did on the first, so
  `(u > 3) and false` is `false`, `(u == 1) or true` and `(u == 1) implies true` are `true`;
  `notEmpty(gear)` is `true` since `[1..*]` guarantees an element; `size(slots)` of a `part
  slots[3]` is `3`; `includes((1, u + 1), 1)` is `true`; a one-valued feature and an enum literal
  keep their definite counts. The result is a first-class value: the CLI and REPL print
  `<undetermined>`, gRPC `Evaluate` carries it as the new `Value.undetermined` arm (its `reason`
  and count bounds) under the `undetermined_value` capability, falling back to an unsupported
  null for a client that does not know the arm, and the Go, Python (`opensysml.Undetermined`,
  whose `bool()` raises), Node, Rust and Java clients decode it. An undetermined constraint or
  requirement condition is still not a verdict: it is reported as `no value`, naming the open
  feature. Nothing changes on an object: a feature it holds nothing for reads `<unset>`, a
  required value that is missing is still `no value for feature`, and `%instantiate` still
  materializes multiplicity minimums. An unresolved name is still `unresolved reference`.
  A model-level read never builds an open collection up to its lower bound: `part many[10001..*]`
  reads `<undetermined>` rather than failing with a multiplicity violation, the values that
  features subsetting the collection contribute are what it certainly holds (`includes(gear,
  fixed)` is `true` for `part fixed :> gear`), and `forAll`, `exists`, `allTrue` and `anyTrue`
  decide from those certain elements (`(1, u)->exists{in x; x == 1}` is `true`,
  `(1, u)->forAll{in x; x > 2}` is `false`) before answering `<undetermined>`. Such a read holds
  nothing, so it is not charged to the element budget.
- **A calc-typed parameter whose value names a calc (`in calc f = twice;`) applies that calc when
  called by its qualified name.** `Apply::f(3.0)` outside a run of `Apply` failed with `calc
  Apply::f has no return expression`; it now applies `twice` as the bare `f(3.0)` does, and a run
  that bound `f` still applies the binding.
