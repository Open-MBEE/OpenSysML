- **Classification against an enumeration is decided by its enumerated values.** An
  enumeration's enumerated values are the only instances it has (SysML v2 §8.3.7
  EnumerationDefinition), so with `enum def Level :> Integer { low = 1; high = 3; }` the shared
  classification rule behind `istype`, `@`, `as` and feature writes now answers `3 istype Level`
  `true`, `2 istype Level` `false`, `3 as Level` the value `Level::high`, `2 as Level` `()`, admits
  `attribute l : Level = 3;` and refuses `= 2` — statically where the value is constant
  (`cannot bind 2 (an Integer) to a feature typed by Level, whose values are Level::low = 1,
  Level::high = 3`), with the write-conformance error at run time otherwise — where every one of
  them was `ErrUndecidedClassification` before. `hastype` still reads the value's own type alone
  (KerML 1.0 §7.4.9.2): a bare `3` is an `Integer` and no `Level`, while a `Level` literal —
  written, held by a `Level` feature or produced by `3 as Level` — is a `Level` and not directly
  an `Integer`; a scalar-valued literal keeps that identity on the scalar it evaluates to, so
  `Level::high hastype Level` is `true` (it was `false`) and `Level::high == 3` still holds. A
  plain `enum def Color { red; green; blue; }` classifies by identity with its literals, and a
  user-defined subtype such as `Even :> Integer` stays undecided against a bare `5`.
