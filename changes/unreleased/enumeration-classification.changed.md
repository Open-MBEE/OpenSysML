- **gRPC: a scalar-valued enumeration literal is an `enum_literal` on the wire.** `Level::high`
  from `enum def Level :> Integer { low = 1; high = 3; }`, a feature `l : Level = Level::high`
  and a successful `3 as Level` used to go out as `int_value: 3`, indistinguishable from a bare
  `3`; they now go out as `enum_literal`, like a plain enumeration's literal. The `EnumLiteral`
  message gains an optional `value` field (additive, so existing decoders keep working) that
  carries the scalar the literal equals, unset for a literal that is only its identity. The
  Python, Node, Java and Rust clients expose it as an optional `value` on their enumeration
  literal type, keeping the literal's identity in `literal_id` alone. A bare `3` that no
  enumeration value holds stays `int_value`.
