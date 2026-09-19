- **A completion enabled by a region's entry stays a run-to-completion step of its own; the
  design that would have drawn its firing inside the entry front is closed without code.**
  Enumerated against the PSSM traces the five tests held on it admit
  (`docs/internals/design/region-order-scheduling.md`), that rule reaches every admitted set
  only by also reaching orders the suite refuses, and the sets turn out to want three
  different things: *Entering 010*, *Entering 011* and *Junction 005* want a UML initial
  transition's *effect* run as part of the region's default entry, which SysML v2 can spell
  only as the effect of a completion transition out of a start state — a translation limit,
  not a scheduling one; folding the effect into the target state's entry action was run
  against the three and refused (it runs the effect on every entry of the state, which
  *Entering 010* refuses, merges two behaviors into one unit, and has no target where the
  initial transition ends at a junction); *History 001-C* and *History 002-B* register
  incompatible orders for structurally identical halves, each contradicting the
  specification's own account of the test (`docs/project/omg-issues.md` gains the entry);
  *Terminate 002*'s remaining trace is a do step against a sibling's entry unit. The one
  runtime gap the enumeration confirms — under `reverse`, `seed:<n>` and `explore`, the
  completion events two regions' entries generate are dispatched in region declaration order
  rather than the order the regions were entered — is recorded for a runtime change of its
  own, since it moves no test alone. The alignment note and the referee record carry the
  adjudication; no bucket, golden or baseline moves (51 pass, 13 fail, 38 not expressible,
  1 differs by design).
