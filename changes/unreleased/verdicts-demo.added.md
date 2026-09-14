- **A worked example of verdict queries.** `examples/verdicts-demo/` holds a rover whose
  constraints, requirement, `satisfy` and verification case are read as a `Verdicts(...)` table:
  twelve rows over the declared object, then the same queries over the object a session holds
  after a drive, with a violated satisfaction beside a passed verification and an undecided
  constraint beside a violated one. The walkthrough spells out what the table checks that
  evaluating one expression does not.
