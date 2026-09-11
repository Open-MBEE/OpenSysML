- **Analysis runs in parallel under `-jobs`, with the same answer whatever the count.**
  `sysml -jobs <n>`, `%jobs <n>` in the REPL and `OPENSYSML_JOBS` (default one per CPU; the
  flag overrides the environment) set how many runs of one check go at once, each on a worker
  of its own — a resolver and semantic model per job over the one loaded model, so no run sees
  another's memo. `-schedule explore` runs its linearizations on a work queue of prefixes
  ordered as the sequential exploration would take them, and reports the outcome table, each
  outcome's witness, the run count and the budget hit `-jobs 1` reports, byte for byte: a
  `runs` budget is a cut in that order, at most `n` runs beyond it are ever started (so an
  exploration performs at most `runs + n` executions), and a run that fails is an outcome of
  the table, as under one job.
  `-engine all` puts the question to its covering engines at once and composes their answers in
  name order; a fault or deadline stops the engines after it, each kept in the plan with the
  bound it reached, and a run serving a universal claim is not cancelled by a witness. A count
  below one, or one that is no integer, is refused before anything runs. `-json` checks carry
  `workers` and `warming` (the milliseconds spent building them) under `plan`; the
  human-readable report does not print them. The gRPC service takes its count from
  `OPENSYSML_JOBS`; no request field changed.
