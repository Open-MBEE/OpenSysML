- **A design note on recording the order of orthogonal regions**
  (`docs/internals/design/region-order-scheduling.md`). The PSSM referee holds ten tests in
  `fail` because the runtime enters and exits a composite state's regions, fires the transitions
  one occurrence selects across regions, and steps a due do action before a dispatch in one fixed
  order, recording no choice for `explore` to vary. The note decides the unit each site draws
  (a firing is not atomic: its source exit, effects and target entry are separate units, argued
  from the KerML library's successions and recorded as an alignment row), the choice kinds with
  their `%trace` and witness lines, what `declared`, `reverse`, `seed:<n>`, `replay:` and
  `explore` do at each site, how a refused replay rolls back, the choice-point budget, and the
  test contract; it stops at two decisions for the maintainers — whether trace goldens recorded
  under the default policy may gain `choice` lines, and two admitted traces of *Transition 017*
  no reading of the model produces. Nothing in the runtime changes; the note exists to be
  reviewed before code is written.
