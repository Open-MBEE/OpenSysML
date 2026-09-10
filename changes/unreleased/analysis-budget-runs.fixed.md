- **The analysis `Budget`'s `Runs` bounds a sweep's rows and a solver's queries, as it does an
  exploration's runs.** `Context.RunSweep` takes the row limit it runs under (the context's
  `OPENSYSML_MAX_SWEEP_RUNS` when none is stated), the `sweep` engine passes the budget's, and the
  `solve` engine asks no more queries than the budget's runs, leaving the set *not covered* with
  the `runs` bound reached when some went unasked. `BudgetOf` fills `Runs` in the unit of the
  question's kind, so a sweep or solver query under an exploring schedule is no longer handed
  the schedule's exploration runs as its bound. No flag, command, RPC, field or line of output
  changed.
