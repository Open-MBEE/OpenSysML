- **Monte Carlo summaries come across as statistics.** A result snapshot of a «SimulationConfig»
  whose target specializes MagicDraw's `MonteCarloAnalysis` records `N`, `Mean`, `Deviation` and
  `OutOfSpec` beside the observed values; the `-migration-results` sidecar now writes them as the
  snapshot's `statistics` of the observable the analysis binds its `Mean` to, standing for `N`
  runs, rather than as one more run, and notes a summary that is incomplete, counts no runs, binds
  no observable, or summarises another configuration's.
- **Every run configuration is compared.** `-compare-results` runs a configuration the tool
  stored no snapshot of and prints its statistics under a `tool (no stored result to compare)`
  row; runs one stating no `numberOfRuns` once, as the tool does, under a note saying so; counts
  the runs a summary stands for and pools raw values with summary means; shows only the
  statistics a summary holds; and notes a summarising snapshot another configuration stores under
  the same name with the same statistics as a likely copy, naming that configuration and its
  result location.
- **A run configuration resolves to an inherited classifier behavior.** The `executionTarget`'s
  classifier behavior is looked up through its generalizations, nearest first, and a test-case
  behavior is performed where its scenario is migrated. A target with no classifier behavior at
  any level whose parts hold constraint properties — a parametric configuration the tool solves
  for values — is reported per constraint property, naming its constraint block and whether the
  block's rule is migrated as a constraint or which call of an opaque rule stops it, in place of
  a blanket refusal.
- **Snapshots of a run on another classifier are set aside.** A result location may hold
  snapshots the tool named after a classifier that is neither the configuration's execution
  target nor a general or special of it; they are of another configuration stored in the same
  package, so they are not read as the configuration's results and the sidecar says so, and the
  sidecar carries the notes saying why a configuration runs no behavior.
- **The clock can tick by a fixed step.** `-clock-step <seconds>` and `%clock-step` make every
  wait of a run — `accept after`, `accept at`, a state's timer, a case's timed step — come due at
  the first multiple of the step not before the instant it ends, as a simulation tool's fixed-step
  clock does; `0`, the default, keeps the continuous clock. The step reaches the run, explore,
  check, sweep and standing engines, the external-engine protocol (`clockStep`) and the gRPC
  handlers as the draw policy does; a witness of a stepped run records `clock steps by <seconds>`
  and replays on it. A migrated «SimulationConfig» stating `startTime` ran on the tool's internal
  clock, so the `-migration-results` sidecar records its `stepSize` in `timeUnit` (`1.0` unless
  stated) as `clockStep`, in seconds, and `-compare-results` runs the configuration on it — a
  unit of no fixed length, a step of zero or less and an unstated unit are noted.
