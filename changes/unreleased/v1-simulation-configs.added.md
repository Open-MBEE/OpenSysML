- **A run resolves its random draws under a policy.** `-draws random|min|max|average` and
  `%draws` state how every `RandomFunctions` call of a run resolves: `random` (the default) draws
  from the seed as before; `min`, `max` and `average` take each call's least, greatest or mean
  value — `uniform(1, 80)` is `1`, `80` or `40.5`; `uniformInteger(1, 6)` averages to `4`;
  `triangular` to `(lo + mode + hi) / 3`; `normal` averages to its mean and has no `min` or
  `max`, which is a typed error naming the call — and need no seed, so a random duration
  becomes a fixed one and `-runs`/`%runs` run without `-seed` under a fixed policy (`%runs <n>
  <action>`, the seed left out). Weighted decisions draw from the seed whatever the policy and
  take their most probable branch unseeded. The policy is a property of the run's context, so
  it reaches the analysis engines, the wire (`"draws":"max"`) and gRPC as the model seed does;
  a witness records it as `draws by max` and `replay:` reproduces the run under it, refusing a
  witness whose draws the recorded policy could not have made. A conformance case pins it with
  `"draws"`.
- **A «Probability» that names a property is a feature reference.** The SysML v1 migration
  writes `@Probability { p = ProbabilityBTOOP; }` when the tag names a property visible from
  the activity or its context block, by name or id, instead of the property's default, so the
  object the behavior runs on decides the branch weights; a tag naming nothing visible, a
  private property, a non-numeric or a multi-valued property is a report entry with the reason.
  A feature-valued weight is type-checked against `Probability::p` where it is written, and the
  checks lowering makes of constants — each in `[0, 1]`, the set summing to one — are made of
  the values read when the decision is reached, each a typed `ErrBranchWeights`.
- **Simulation run configurations migrate.** A MagicDraw «SimulationConfig», recognised by its
  profile's provenance, becomes an `action def` holding its `executionTarget` individual as
  `part target` and performing the target's classifier behavior on it, annotated
  `@Simulation::Configuration { runs = …; draws = DrawPolicy::…; timeVariable = …; startTime = …;
  stepSize = …; timeUnit = …; parallelForks = …; }` — a new non-normative library beside
  `Stochastic` that records how the tool ran the behavior and applies none of it — with every
  setting of no v2 meaning kept in a comment. The configuration, its target and result
  instances and the probability edges it reads are mapped rather than unmapped; a target the
  migration did not write, one that is no part, a state machine or a classifier with no
  behavior is reported with the reason. The outcome of an action spells `<part>.<attribute>`
  for the one object each of its own parts denotes, so `-observe target.duration` reads
  the target's attribute after the run.
- **The tool's results come across, and a harness compares them.** `-migration-results
  <file>` writes a JSON sidecar indexing the tool's result-snapshot instances — classifier-less
  or typed, found under each configuration's `resultLocation` or result package — per
  configuration and observable; `sysml <migrated>.sysml -compare-results <file> [-action
  <configuration>...] [-runs <n>] [-draws <policy>] [-seed <s>] [-observe <stored>[=<feature>]...]`
  runs each configuration with its recorded run count and policy, or the ones given, and
  reports the tool's and OpenSysML's min, mean, p50, p90 and max of each observable with the
  relative difference; a configuration with no stored snapshots, a stored observable no run
  holds and a non-numeric one are reported, never left out. The numbers are reported as run.
