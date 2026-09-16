- **A model states its own odds.** Two non-normative OpenSysML libraries add what SysML v2 has no
  notation for: `Stochastic::Probability` weights the successions out of a decision node
  (`first d then fast { @Probability { p = 0.7; } }`), and `RandomFunctions` declares `uniform`,
  `uniformInteger`, `triangular` and `normal`, so `attribute d : Real = uniform(0.0, 1.0);` and
  `accept after uniform(1, 80) [s]` run. The weights are validated at lowering — every succession
  out of a decision weighted or none, each in `[0, 1]`, constant weights summing to one — and a
  `Probability` on a state transition is refused rather than ignored. The flow among an analysis or verification case's steps reads the same weights, and the exported action graph carries each weight beside its edge. Modeled randomness is a
  stream of its own, apart from the token-shuffle stream: `-seed <n>` and `%seed <n>` fix it
  whatever the scheduling policy, `seed:<n>` seeds it too when no model seed is set, an unseeded
  weighted decision under `declared` or `reverse` takes its most probable branch, and an
  unseeded random function is refused naming the flags that seed it. The scheduling choice points
  the spec leaves open — token, write, region and due order — stay unweighted, and `explore` and
  `check` still enumerate and search weighted branches as a set. Every draw is recorded in the
  witness (`draw uniform(0.0, 1.0) = 0.7748…`) beside the weighted pick, so `%replay` and
  `-schedule replay:<file>` reproduce a run exactly and refuse a witness whose draws they cannot
  consume; the trace reports each weighted decision with its weights and its draw.
- **Monte Carlo runs.** `%runs <n> <seed> <action> [<observable>...]` and
  `sysml -action <a> -runs <n> -seed <s> [-observe <f>]` run an action `n` times, each under a
  model seed derived from the seed and the run number, on the sweep machinery, and report the
  table of the observables — every feature the action holds and `clock` when none is named —
  then each numeric observable's min, mean, max, p50, p90 and a compact histogram, as
  `%samples` reports a table. Runs are reproducible for a seed and distinct across seeds.
