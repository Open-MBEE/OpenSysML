- **`*` is a value.** In an expression position `*` evaluates to the unbounded value: it exceeds
  every finite Integer, Real and Natural, equals itself, prints as `*` in the REPL and in traces,
  and crosses gRPC on its own `Value.infinity` arm under the `infinity_value` capability — never
  as the ordinary string `"*"`. Arithmetic over it is refused with a typed error naming the
  operation rather than an infinity or a NaN.
- **`elem.metadata` reads an element's metadata.** `ref.metadata` yields the metadata annotating
  the element as a sequence of metadata instances, in declaration order, with the feature values
  the annotation body binds and the metadata type's own defaults where it binds none. An element
  with no metadata yields the empty sequence, and reading metadata off a value is a typed error.
