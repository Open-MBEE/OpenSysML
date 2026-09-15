- **The PSSM referee's translation carries the values a test's constructor writes.** A test
  class whose `<Class>$factory` activity assigns a literal to an attribute of the new instance
  (*Join003*'s `value = 15`, read by the guard of the join's outgoing transition) lost the
  assignment: the attribute was declared without a value and the run failed with `no value for
  feature value` before reaching the guard. The literal is now the attribute's initial value; a
  constructor that does anything else — writes a feature the class does not own, writes
  something other than the new instance, or computes a value — is refused as untranslatable
  rather than dropped. No bucket count moves: *Join003* still fails, now at the join itself
  (`docs/project/pssm-referee.md`).
