- A `[0..*]` feature holding one value now denotes that value wherever one value is
  taken — an operator operand, a `[1]` parameter of a user calc or library function, a
  cast or `istype`/`hastype` operand — since a feature's values are a sequence its
  multiplicity constrains (KerML §7.3.4.1, §7.4.12). `SampledFunctions::SamplePair`
  arithmetic and `interpolateLinear` on the library's own examples evaluate, as does
  `q.zs + 1.0` with `zs : Real[0..*] = (2.5)`; a collection of several values is refused
  as before, and multiplicity checks are unchanged.
