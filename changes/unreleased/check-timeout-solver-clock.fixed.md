- **`-check-timeout` (`%check-bounds timeout=`) is the `smt` engine's solver clock as well as
  the plan's.** Each solver query of a check runs under the check's timeout in place of
  `OPENSYSML_SMT_TIMEOUT`, so a check told it may run for `2m` is no longer left *not covered*
  by a query the solver's own 10 s default cut short; the `solver` bound the result names is
  the clock the query ran under. Without a timeout the queries keep `OPENSYSML_SMT_TIMEOUT`.
