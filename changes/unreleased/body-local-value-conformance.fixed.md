- **A declaration local to a behavior body answers to its declared type, multiplicity and
  uniqueness when its initial value is bound**, as a parameter, a `return` and a namespace-level
  declaration already do (KerML 1.0 §7.3.4, "the values of a feature are instances of its types").
  With `enum def Level :> Integer { low = 1; high = 3; }`,
  `calc def BodyOnly { in n : Integer; attribute l : Level = n; return : Integer = l + 0; }`
  answered `BodyOnly(2)` with `2`; it is now the write's `type mismatch: cannot write 2 (an Integer)
  to a feature typed by Level`, and `BodyOnly(3)` holds `l` as `Level::high`. A local stating a
  multiplicity (`attribute xs : Integer[2] = (n, n + 1, n + 2);`) is a `multiplicity violation`
  where its value's count falls outside it, a unique multi-valued local (`Integer[*] = (n, n + 1, n)`)
  a `uniqueness violation`; one stating no multiplicity keeps the count it is given and one declaring
  no type holds anything. The rule holds in a calc, action or constraint body, an `if` or loop block
  and a collection-expression body, and on the compiled tier, which checks a scalar or collection
  local the same way and declines to compile an enumeration-typed one rather than answer differently.
