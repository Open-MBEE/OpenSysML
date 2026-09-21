- **A `perform action` usage naming nothing performs itself.** Per SysML v2
  §8.3.16 (`EventOccurrenceUsage::eventOccurrence` — the reference feature of the
  owned reference subsetting, or the usage itself when there is none) and §8.3.17,
  `perform action boost { in amount = level; }` and `perform action idle;` are
  action usages with their own (possibly empty) body, not references to one held
  elsewhere; instantiation no longer fails with `classifier behavior names no
  body` on them, and the body's `in` members bind the performance's parameters.
  A `perform` declaration that does name an element — the `perform a;` reference
  form, a `references`/`::>` clause, or a typing — that resolves to no behavior
  body is still reported.
