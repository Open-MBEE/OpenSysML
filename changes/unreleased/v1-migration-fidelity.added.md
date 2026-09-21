- **Monte Carlo summaries come across as statistics.** A result snapshot of a «SimulationConfig»
  whose target specializes MagicDraw's `MonteCarloAnalysis` records `N`, `Mean`, `Deviation` and
  `OutOfSpec` beside the observed values; the `-migration-results` sidecar now writes them as the
  snapshot's `statistics` of the observable the analysis binds its `Mean` to, standing for `N`
  runs, rather than as one more run — `deviation` and `outOfSpec` only when the snapshot records
  them, so a missing deviation is not a zero — and notes a summary that is incomplete, counts
  no runs or more than a count holds, binds no observable, states an `OutOfSpec` that is no
  count of its runs, or summarises another configuration's.
- **Every run configuration is compared.** `-compare-results` runs a configuration the tool
  stored no snapshot of and prints its statistics under a `tool (no stored result to compare)`
  row; runs one stating no `numberOfRuns` once, as the tool does, under a note saying so; counts
  the runs a summary stands for and pools raw values with summary means; shows only the
  statistics a summary holds; and notes a summarising snapshot another configuration stores under
  the same name with the same statistics as a likely copy, naming that configuration and its
  result location; and notes summaries of one observable whose means lie more than three
  standard errors apart, which cannot be of runs of one and the same model, so the pooled mean
  they are compared by blends them.
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
  clock, so the `-migration-results` sidecar records its `stepSize` in `timeUnit` (`1.0` and the
  millisecond, the tool's defaults, unless stated; `(endTime − startTime) / numberOfSteps` when
  those two stand in for the step) as `clockStep`, in seconds, and
  `-compare-results` runs the configuration on it — a unit of no fixed length, a step of zero or
  less, one of more seconds than a number holds or fewer than it tells from none, and an unstated
  unit are noted. The tool's clock started at `startTime` and a run's starts at 0, so a
  `startTime` other than 0 is noted, in the report, the sidecar and the comparison, as offsetting
  every instant read on the clock.
- **A script's console print is left out.** A `print(…)`, `println(…)` or `System.out.println(…)`
  statement of an opaque body writes to the tool's console and changes nothing of the model, so
  the SysML v1 migration leaves it out of the translation, keeps the other statements of the body,
  and notes each print left out as an approximation; a body of prints alone is an empty action.
  A print whose argument assigns, counts, deletes, constructs or calls anything but a function of the table computing
  a value (a Java `equals` counts only on a receiver known to be a string; any other type's is
  that type's own method) could change the model, so it is refused rather than left out; a call
  not in the table, or a print used as a value, is refused as before.
